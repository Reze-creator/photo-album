package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"photo-album/backend/internal/api"
	"photo-album/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := envOr("ALBUM_ADDR", "127.0.0.1:8080")
	dataDir := envOr("ALBUM_DATA_DIR", "data")
	uploadsDir := filepath.Join(dataDir, "uploads")
	for _, folder := range []string{"avatars", "photos"} {
		if err := os.MkdirAll(filepath.Join(uploadsDir, folder), 0o700); err != nil {
			return err
		}
	}
	key, err := jwtSigningKey(dataDir)
	if err != nil {
		return fmt.Errorf("加载 JWT 密钥: %w", err)
	}
	db, err := store.Open(filepath.Join(dataDir, "album.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	gin.SetMode(envOr("GIN_MODE", gin.ReleaseMode))
	server := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(db, uploadsDir, key),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	done := make(chan error, 1)
	go func() {
		log.Printf("相册 API 监听 %s，数据目录 %s", addr, dataDir)
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// 默认把随机密钥保存在数据目录，服务重启后未过期的 Token 仍可使用。
func jwtSigningKey(dataDir string) ([]byte, error) {
	if key := os.Getenv("ALBUM_JWT_SECRET"); key != "" {
		if len(key) < 32 {
			return nil, errors.New("ALBUM_JWT_SECRET 至少需要 32 字节")
		}
		return []byte(key), nil
	}
	path := filepath.Join(dataDir, "jwt.key")
	stored, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, err
		}
		_, writeErr := fmt.Fprintln(file, hex.EncodeToString(key))
		if err := errors.Join(writeErr, file.Close()); err != nil {
			_ = os.Remove(path)
			return nil, err
		}
		return key, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(stored)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("jwt.key 格式错误")
	}
	return key, nil
}
