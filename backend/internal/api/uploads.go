package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
)

const maxAvatarSize int64 = 2 << 20
const maxPhotoSize int64 = 10 << 20

var imageName = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png|gif)$`)

func parseUploadForm(c *gin.Context, fileLimit int64) bool {
	// 整个请求额外预留空间给 multipart 字段，小体积表单驻留内存。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, fileLimit+(64<<10))
	if err := c.Request.ParseMultipartForm(512 << 10); err != nil {
		var limitErr *http.MaxBytesError
		if errors.As(err, &limitErr) {
			fail(c, http.StatusRequestEntityTooLarge, "上传文件超过大小限制")
		} else {
			fail(c, http.StatusBadRequest, "请使用 multipart/form-data 上传图片")
		}
		return false
	}
	return true
}

type savedImage struct {
	path string
	url  string
	size int64
}

func (f savedImage) remove() {
	if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("清理上传文件失败: %v", err)
	}
}

func (s *server) saveUpload(c *gin.Context, field, folder string, limit int64) (savedImage, bool) {
	files := c.Request.MultipartForm.File[field]
	if len(files) != 1 || files[0].Size == 0 {
		fail(c, http.StatusBadRequest, "请选择一张图片")
		return savedImage{}, false
	}
	header := files[0]
	if header.Size > limit {
		fail(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("图片大小不能超过 %d MB", limit>>20))
		return savedImage{}, false
	}
	source, err := header.Open()
	if err != nil {
		internalError(c, err)
		return savedImage{}, false
	}
	defer source.Close()

	// 从文件内容识别格式，忽略原文件名、扩展名和客户端传来的 MIME。
	config, format, err := image.DecodeConfig(source)
	extensions := map[string]string{"jpeg": ".jpg", "png": ".png", "gif": ".gif"}
	ext, supported := extensions[format]
	if err != nil || !supported || config.Width < 1 || config.Height < 1 {
		fail(c, http.StatusBadRequest, "请选择有效的 JPG、PNG 或 GIF 图片")
		return savedImage{}, false
	}
	if config.Width > 12000 || config.Height > 12000 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		fail(c, http.StatusBadRequest, "图片尺寸过大")
		return savedImage{}, false
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		internalError(c, err)
		return savedImage{}, false
	}

	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		internalError(c, err)
		return savedImage{}, false
	}
	name := hex.EncodeToString(id[:]) + ext
	result := savedImage{
		path: filepath.Join(s.uploadsDir, folder, name),
		url:  "/uploads/" + folder + "/" + name,
	}
	// 排他创建保证极小概率的命名冲突也不会覆盖已有文件。
	destination, err := os.OpenFile(result.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		internalError(c, err)
		return savedImage{}, false
	}
	result.size, err = io.Copy(destination, io.LimitReader(source, limit+1))
	closeErr := destination.Close()
	if err = errors.Join(err, closeErr); err != nil {
		result.remove()
		internalError(c, err)
		return savedImage{}, false
	}
	if result.size > limit {
		result.remove()
		fail(c, http.StatusRequestEntityTooLarge, "上传文件超过大小限制")
		return savedImage{}, false
	}
	return result, true
}

func (s *server) avatarFile(c *gin.Context) {
	s.serveImage(c, "avatars", c.Param("name"))
}

func (s *server) photoFile(c *gin.Context) {
	// 先检查权限再读取文件，禁止通过已知 URL 绕过相册鉴权。
	c.Header("Cache-Control", "private, no-store")
	name := c.Param("name")
	album, err := s.store.AlbumForPhoto(c.Request.Context(), "/uploads/photos/"+name)
	if err != nil {
		lookupError(c, err, "照片不存在")
		return
	}
	if allowRead(c, album) {
		s.serveImage(c, "photos", name)
	}
}

func (s *server) serveImage(c *gin.Context, folder, name string) {
	// 只接受本服务生成的文件名，保证路径不会指向其他目录。
	if !imageName.MatchString(name) {
		fail(c, http.StatusNotFound, "图片不存在")
		return
	}
	file, err := os.Open(filepath.Join(s.uploadsDir, folder, name))
	if errors.Is(err, os.ErrNotExist) {
		fail(c, http.StatusNotFound, "图片不存在")
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		internalError(c, err)
		return
	}
	http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
}
