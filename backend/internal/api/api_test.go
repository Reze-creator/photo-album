package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"photo-album/backend/internal/store"
)

// 用两个真实用户验证私有照片列表、原图和上传的权限，防止某个入口漏检。
func TestAlbumAccess(t *testing.T) {
	origin := os.Getenv("ALBUM_TEST_ORIGIN")
	if origin == "" {
		gin.SetMode(gin.TestMode)
		dir := t.TempDir()
		for _, folder := range []string{"avatars", "photos"} {
			if err := os.Mkdir(filepath.Join(dir, folder), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		db, err := store.Open(filepath.Join(dir, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		host := httptest.NewServer(NewRouter(db, dir, bytes.Repeat([]byte{'k'}, 32)))
		t.Cleanup(host.Close)
		origin = host.URL
	}
	client := &http.Client{Timeout: 10 * time.Second}
	reply := func(method, path, token string, body io.Reader, contentType string, expected int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, origin+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		payload, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != expected {
			t.Fatalf("%s %s 返回 %d，期望 %d，响应 %s", method, path, res.StatusCode, expected, payload)
		}
		return payload
	}
	picture := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			picture.Set(x, y, color.RGBA{20, uint8(90 + 4*x), 105, 255})
		}
	}
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, picture); err != nil {
		t.Fatal(err)
	}

	photoForm := func(fields map[string]string, field string, content []byte) (*bytes.Buffer, string) {
		t.Helper()
		body := &bytes.Buffer{}
		form := multipart.NewWriter(body)
		for key, value := range fields {
			if err := form.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		part, err := form.CreateFormFile(field, "example.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		return body, form.FormDataContentType()
	}
	password := "sample-test-password"
	ownerName := "owner-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	otherName := "visitor-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	register := func(username string) string {
		fields := map[string]string{"username": username, "password": password}
		body, kind := photoForm(fields, "avatar", pngBytes.Bytes())
		user := decodeData[store.Profile](t, reply("POST", "/api/v1/register", "", body, kind, 201))
		if user.Username != username || !bytes.Equal(reply("GET", user.AvatarURL, "", nil, "", 200), pngBytes.Bytes()) {
			t.Fatal("头像或用户信息不正确")
		}
		if username == ownerName {
			body, kind = photoForm(fields, "avatar", pngBytes.Bytes())
			reply("POST", "/api/v1/register", "", body, kind, 409)
		}
		credentials := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
		login := decodeData[struct{ Token string }](t, reply("POST", "/api/v1/login", "", bytes.NewBufferString(credentials), "application/json", 200))
		if login.Token == "" {
			t.Fatal("缺少登录 Token")
		}
		return login.Token
	}
	owner, visitor := register(ownerName), register(otherName)
	reply("GET", "/api/v1/health", "", nil, "", 200)
	reply("GET", "/api/v1/albums", "", nil, "", 401)
	reply("GET", "/api/v1/albums", "invalid.token", nil, "", 401)
	create := func(name string, public bool) store.Album {
		input := fmt.Sprintf(`{"name":%q,"is_public":%t}`, name, public)
		return decodeData[store.Album](t, reply("POST", "/api/v1/albums", owner, bytes.NewBufferString(input), "application/json", 201))
	}
	private := create("仅自己可见", false)
	public := create("公开测试相册", true)
	myPage := decodeData[store.Page[store.Album]](t, reply("GET", "/api/v1/albums?page=2&page_size=1", owner, nil, "", 200))
	if myPage.Total != 2 || len(myPage.Items) != 1 || myPage.Items[0].ID != private.ID {
		t.Fatal("我的相册分页结果不正确")
	}
	plaza := decodeData[store.Page[store.Album]](t, reply("GET", "/api/v1/albums/public?page_size=50", "", nil, "", 200))
	found := false
	for _, album := range plaza.Items {
		if album.ID == private.ID {
			t.Fatal("公开广场泄露私有相册")
		}
		if album.ID == public.ID {
			found = album.Creator.Username == ownerName && album.Creator.AvatarURL != ""
		}
	}
	if !found {
		t.Fatal("公开广场缺少正确的创建者信息")
	}
	reply("GET", "/api/v1/albums/public?page_size=51", "", nil, "", 400)

	for _, album := range []store.Album{private, public} {
		detailPath := fmt.Sprintf("/api/v1/albums/%d", album.ID)
		photosPath := detailPath + "/photos"
		form, kind := photoForm(nil, "photo", pngBytes.Bytes())
		reply("POST", photosPath, visitor, form, kind, 403)
		form, kind = photoForm(nil, "photo", []byte("not an image"))
		reply("POST", photosPath, owner, form, kind, 400)
		form, kind = photoForm(nil, "photo", pngBytes.Bytes())
		photo := decodeData[store.Photo](t, reply("POST", photosPath, owner, form, kind, 201))
		form, kind = photoForm(nil, "photo", pngBytes.Bytes())
		second := decodeData[store.Photo](t, reply("POST", photosPath, owner, form, kind, 201))
		if photo.FileURL == second.FileURL {
			t.Fatal("上传覆盖了原照片")
		}
		photoPage := decodeData[store.Page[store.Photo]](t, reply("GET", photosPath+"?page=2&page_size=1", owner, nil, "", 200))
		if photoPage.Total != 2 || len(photoPage.Items) != 1 || photoPage.Items[0].ID != photo.ID {
			t.Fatal("照片分页结果不正确")
		}
		if !bytes.Equal(reply("GET", photo.FileURL, owner, nil, "", 200), pngBytes.Bytes()) {
			t.Fatal("原图字节不匹配")
		}
		if album.IsPublic {
			reply("GET", detailPath, "", nil, "", 200)
			reply("GET", photosPath, "", nil, "", 200)
			reply("GET", photo.FileURL, "", nil, "", 200)
		} else {
			reply("GET", detailPath, visitor, nil, "", 403)
			reply("GET", photosPath, visitor, nil, "", 403)
			reply("GET", photo.FileURL, visitor, nil, "", 403)
			reply("HEAD", photo.FileURL, visitor, nil, "", 403)
			reply("GET", photosPath, "", nil, "", 401)
			reply("GET", photo.FileURL, "", nil, "", 401)
		}
	}
}

func decodeData[T any](t *testing.T, payload []byte) T {
	t.Helper()
	var response struct {
		Code int
		Data T
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("响应 code 为 %d", response.Code)
	}
	return response.Data
}
