package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"photo-album/backend/internal/store"
)

type server struct {
	store      *store.Store
	uploadsDir string
	jwtKey     []byte
}

// NewRouter 统一注册路由。公开读取可选登录，写入和我的相册必须登录。
func NewRouter(db *store.Store, uploadsDir string, jwtKey []byte) *gin.Engine {
	s := &server{store: db, uploadsDir: uploadsDir, jwtKey: jwtKey}
	router := gin.Default()
	// 本项目只接受本机代理，访问控制也不依赖客户端 IP。
	_ = router.SetTrustedProxies(nil)
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	router.HandleMethodNotAllowed = true
	router.NoRoute(func(c *gin.Context) { fail(c, http.StatusNotFound, "请求的资源不存在") })
	router.NoMethod(func(c *gin.Context) { fail(c, http.StatusMethodNotAllowed, "请求方法不支持") })

	v1 := router.Group("/api/v1")
	v1.GET("/health", s.health)
	v1.POST("/register", s.register)
	v1.POST("/login", s.login)
	v1.GET("/albums/public", s.publicAlbums)

	read := v1.Group("/albums", s.authenticate(false))
	read.GET("/:id", s.album)
	read.GET("/:id/photos", s.photos)

	protected := v1.Group("/albums", s.authenticate(true))
	protected.POST("", s.createAlbum)
	protected.GET("", s.myAlbums)
	protected.POST("/:id/photos", s.uploadPhoto)

	router.GET("/uploads/avatars/:name", s.avatarFile)
	media := router.Group("/uploads/photos", s.authenticate(false))
	media.GET("/:name", s.photoFile)
	media.HEAD("/:name", s.photoFile)
	return router
}

type response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func success(c *gin.Context, status int, data any) {
	c.JSON(status, response{Code: 0, Message: "ok", Data: data})
}

func fail(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, response{Code: status, Message: message})
}

func internalError(c *gin.Context, err error) {
	log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	fail(c, http.StatusInternalServerError, "服务暂时无法完成请求")
}

func lookupError(c *gin.Context, err error, message string) {
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, message)
		return
	}
	internalError(c, err)
}

func (s *server) health(c *gin.Context) {
	if err := s.store.Ping(c.Request.Context()); err != nil {
		log.Printf("健康检查失败: %v", err)
		fail(c, http.StatusServiceUnavailable, "数据库暂不可用")
		return
	}
	c.Header("Cache-Control", "no-store")
	success(c, http.StatusOK, gin.H{"status": "ok"})
}

func readJSON(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(dst); err != nil {
		var limitErr *http.MaxBytesError
		if errors.As(err, &limitErr) {
			fail(c, http.StatusRequestEntityTooLarge, "请求体过大")
		} else {
			fail(c, http.StatusBadRequest, "JSON 格式或字段类型不正确")
		}
		return false
	}
	return true
}

func pagination(c *gin.Context) (int, int, bool) {
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	// 约束分页参数，避免异常输入造成整数溢出或一次查询过多记录。
	if pageErr != nil || sizeErr != nil || page < 1 || page > 1_000_000 || size < 1 || size > 50 {
		fail(c, http.StatusBadRequest, "page 必须为 1 到 1000000 的整数，page_size 必须为 1 到 50 的整数")
		return 0, 0, false
	}
	return page, size, true
}

const tokenLifetime = 24 * time.Hour
