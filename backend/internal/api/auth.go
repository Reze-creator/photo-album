package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"photo-album/backend/internal/store"
)

const tokenIssuer = "photo-album"
const viewerKey = "viewer_id"

func (s *server) authenticate(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" && !required {
			c.Next()
			return
		}
		scheme, value, _ := strings.Cut(header, " ")
		if !strings.EqualFold(scheme, "Bearer") || value == "" {
			fail(c, http.StatusUnauthorized, "请先登录")
			return
		}
		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(value, claims, func(_ *jwt.Token) (any, error) {
			return s.jwtKey, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
		id, idErr := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil || !token.Valid || idErr != nil || id < 1 {
			fail(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		c.Set(viewerKey, id)
		c.Next()
	}
}

func viewer(c *gin.Context) int64 { return c.GetInt64(viewerKey) }

func (s *server) register(c *gin.Context) {
	if !parseUploadForm(c, maxAvatarSize) {
		return
	}
	defer c.Request.MultipartForm.RemoveAll()

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if length := utf8.RuneCountInString(username); length < 2 || length > 30 {
		fail(c, http.StatusBadRequest, "用户名需为 2–30 个字符")
		return
	}
	if utf8.RuneCountInString(password) < 8 || len(password) > 72 {
		fail(c, http.StatusBadRequest, "密码至少 8 个字符，且不超过 72 字节")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		internalError(c, err)
		return
	}

	avatar, ok := s.saveUpload(c, "avatar", "avatars", maxAvatarSize)
	if !ok {
		return
	}
	profile, err := s.store.CreateUser(c.Request.Context(), username, string(hash), avatar.url)
	if err != nil {
		// 插入失败时清理本次落盘的头像，避免留下孤立文件。
		avatar.remove()
		if errors.Is(err, store.ErrUsernameTaken) {
			fail(c, http.StatusConflict, "用户名已被注册")
		} else {
			internalError(c, err)
		}
		return
	}
	success(c, http.StatusCreated, profile)
}

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *server) login(c *gin.Context) {
	var input loginInput
	if !readJSON(c, &input) {
		return
	}
	user, err := s.store.UserByName(c.Request.Context(), strings.TrimSpace(input.Username))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		internalError(c, err)
		return
	}
	if err != nil || len(input.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		fail(c, http.StatusUnauthorized, "用户名或密码不正确")
		return
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   strconv.FormatInt(user.ID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtKey)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	success(c, http.StatusOK, gin.H{"token": token, "user": user.Profile})
}
