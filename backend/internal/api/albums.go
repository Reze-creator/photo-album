package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"photo-album/backend/internal/store"
)

type albumInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

func (s *server) createAlbum(c *gin.Context) {
	var input albumInput
	if !readJSON(c, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if count := utf8.RuneCountInString(input.Name); count < 1 || count > 60 {
		fail(c, http.StatusBadRequest, "相册名称需为 1–60 个字符")
		return
	}
	if utf8.RuneCountInString(input.Description) > 300 {
		fail(c, http.StatusBadRequest, "相册描述不能超过 300 个字符")
		return
	}
	album, err := s.store.CreateAlbum(c.Request.Context(), viewer(c), input.Name, input.Description, input.IsPublic)
	if err != nil {
		internalError(c, err)
		return
	}
	success(c, http.StatusCreated, album)
}

func (s *server) myAlbums(c *gin.Context)     { s.listAlbums(c, viewer(c)) }
func (s *server) publicAlbums(c *gin.Context) { s.listAlbums(c, 0) }

func (s *server) listAlbums(c *gin.Context, userID int64) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	result, err := s.store.Albums(c.Request.Context(), userID, page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (s *server) album(c *gin.Context) {
	album, ok := s.loadAlbum(c)
	if !ok || !allowRead(c, album) {
		return
	}
	success(c, http.StatusOK, album)
}

func (s *server) photos(c *gin.Context) {
	album, ok := s.loadAlbum(c)
	if !ok || !allowRead(c, album) {
		return
	}
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	result, err := s.store.Photos(c.Request.Context(), album.ID, page, size)
	if err != nil {
		internalError(c, err)
		return
	}
	success(c, http.StatusOK, result)
}

func (s *server) uploadPhoto(c *gin.Context) {
	album, ok := s.loadAlbum(c)
	if !ok {
		return
	}
	if album.Creator.ID != viewer(c) {
		fail(c, http.StatusForbidden, "只有相册主人可以上传照片")
		return
	}
	if !parseUploadForm(c, maxPhotoSize) {
		return
	}
	defer c.Request.MultipartForm.RemoveAll()

	file, ok := s.saveUpload(c, "photo", "photos", maxPhotoSize)
	if !ok {
		return
	}
	photo, err := s.store.CreatePhoto(c.Request.Context(), album.ID, file.url, file.size)
	if err != nil {
		file.remove()
		internalError(c, err)
		return
	}
	success(c, http.StatusCreated, photo)
}

func (s *server) loadAlbum(c *gin.Context) (store.Album, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		fail(c, http.StatusBadRequest, "相册 ID 无效")
		return store.Album{}, false
	}
	album, err := s.store.Album(c.Request.Context(), id)
	if err != nil {
		lookupError(c, err, "相册不存在")
		return store.Album{}, false
	}
	return album, true
}

// allowRead 同时服务相册详情、照片列表和原图，三处执行同一套权限规则。
func allowRead(c *gin.Context, album store.Album) bool {
	if album.IsPublic {
		return true
	}
	c.Header("Cache-Control", "private, no-store")
	if viewer(c) == album.Creator.ID {
		return true
	}
	if viewer(c) == 0 {
		fail(c, http.StatusUnauthorized, "请登录后查看私有相册")
	} else {
		fail(c, http.StatusForbidden, "只有相册主人可以查看私有相册")
	}
	return false
}
