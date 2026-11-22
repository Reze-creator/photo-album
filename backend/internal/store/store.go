package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	sqlite "modernc.org/sqlite"
	"modernc.org/sqlite/lib"
)

//go:embed schema.sql
var schema embed.FS

var ErrUsernameTaken = errors.New("用户名已存在")

// Store 负责数据库读写。权限由 API 层根据查询结果统一判断。
type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// 单连接适合本地作业，避免并发写锁争用。启用外键后再建表。
	db.SetMaxOpenConns(1)
	ddl, err := schema.ReadFile("schema.sql")
	if err == nil {
		_, err = db.Exec(string(ddl))
	}
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化 SQLite: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

type Profile struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type User struct {
	Profile
	PasswordHash string `json:"-"`
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, avatarURL string) (Profile, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password, avatar_url) VALUES (?, ?, ?)`, username, passwordHash, avatarURL)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return Profile{}, ErrUsernameTaken
		}
		return Profile{}, err
	}
	id, err := result.LastInsertId()
	return Profile{ID: id, Username: username, AvatarURL: avatarURL}, err
}

func (s *Store) UserByName(ctx context.Context, username string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password, avatar_url FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.AvatarURL)
	return user, err
}

type Album struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	IsPublic    bool    `json:"is_public"`
	CreatedAt   string  `json:"created_at"`
	Creator     Profile `json:"creator"`
	PhotoCount  int64   `json:"photo_count"`
	CoverURL    string  `json:"cover_url,omitempty"`
}

const albumSelect = `SELECT a.id, a.name, a.description, a.is_public, a.created_at,
    u.id, u.username, u.avatar_url,
    (SELECT COUNT(*) FROM photos p WHERE p.album_id = a.id),
    COALESCE((SELECT p.file_path FROM photos p WHERE p.album_id = a.id ORDER BY p.id DESC LIMIT 1), '')
    FROM albums a JOIN users u ON u.id = a.user_id`

type rowScanner interface{ Scan(dest ...any) error }

func scanAlbum(row rowScanner) (Album, error) {
	var a Album
	err := row.Scan(&a.ID, &a.Name, &a.Description, &a.IsPublic, &a.CreatedAt,
		&a.Creator.ID, &a.Creator.Username, &a.Creator.AvatarURL, &a.PhotoCount, &a.CoverURL)
	return a, err
}

func (s *Store) CreateAlbum(ctx context.Context, userID int64, name, description string, isPublic bool) (Album, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO albums (user_id, name, description, is_public) VALUES (?, ?, ?, ?)`,
		userID, name, description, isPublic)
	if err != nil {
		return Album{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Album{}, err
	}
	return s.Album(ctx, id)
}

func (s *Store) Album(ctx context.Context, id int64) (Album, error) {
	return scanAlbum(s.db.QueryRowContext(ctx, albumSelect+` WHERE a.id = ?`, id))
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// userID 为 0 表示查询广场，其他值表示查询该用户的相册。
func (s *Store) Albums(ctx context.Context, userID int64, page, size int) (Page[Album], error) {
	where := ` WHERE a.is_public = 1`
	args := []any{}
	if userID != 0 {
		where = ` WHERE a.user_id = ?`
		args = append(args, userID)
	}
	return queryPage(ctx, s.db, `SELECT COUNT(*) FROM albums a`+where,
		albumSelect+where+` ORDER BY a.id DESC LIMIT ? OFFSET ?`, args, page, size, scanAlbum)
}

type Photo struct {
	ID        int64  `json:"id"`
	AlbumID   int64  `json:"album_id"`
	FileURL   string `json:"file_url"`
	FileSize  int64  `json:"file_size"`
	CreatedAt string `json:"created_at"`
}

const photoSelect = `SELECT id, album_id, file_path, file_size, created_at FROM photos`

func scanPhoto(row rowScanner) (Photo, error) {
	var p Photo
	err := row.Scan(&p.ID, &p.AlbumID, &p.FileURL, &p.FileSize, &p.CreatedAt)
	return p, err
}

func (s *Store) CreatePhoto(ctx context.Context, albumID int64, fileURL string, fileSize int64) (Photo, error) {
	return scanPhoto(s.db.QueryRowContext(ctx,
		`INSERT INTO photos (album_id, file_path, file_size) VALUES (?, ?, ?)
         RETURNING id, album_id, file_path, file_size, created_at`, albumID, fileURL, fileSize))
}

func (s *Store) Photos(ctx context.Context, albumID int64, page, size int) (Page[Photo], error) {
	return queryPage(ctx, s.db, `SELECT COUNT(*) FROM photos WHERE album_id = ?`,
		photoSelect+` WHERE album_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		[]any{albumID}, page, size, scanPhoto)
}

// 直接请求图片时，按路径查回所属相册，复用详情页的访问规则。
func (s *Store) AlbumForPhoto(ctx context.Context, fileURL string) (Album, error) {
	var a Album
	err := s.db.QueryRowContext(ctx, `SELECT a.id, a.user_id, a.is_public
        FROM photos p JOIN albums a ON a.id = p.album_id WHERE p.file_path = ?`, fileURL).
		Scan(&a.ID, &a.Creator.ID, &a.IsPublic)
	return a, err
}

func queryPage[T any](ctx context.Context, db *sql.DB, countSQL, pageSQL string, args []any,
	page, size int, scan func(rowScanner) (T, error)) (Page[T], error) {
	result := Page[T]{Items: []T{}, Page: page, PageSize: size}
	// 在同一事务执行计数和查询，避免并发上传时总数与结果来自不同快照。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, countSQL, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, pageSQL, append(args, size, (page-1)*size)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, tx.Commit()
}
