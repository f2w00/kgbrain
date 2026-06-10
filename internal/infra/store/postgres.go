package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"

	"kgbrain/internal/resource"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres 使用数据库资源配置打开业务 Postgres 连接。
func OpenPostgres(r *resource.DatabaseResource) (*sql.DB, error) {
	if r == nil {
		return nil, fmt.Errorf("database resource is required")
	}
	sslMode := r.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	uri := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(r.User, r.Password),
		Host:   r.Host + ":" + strconv.Itoa(r.Port),
		Path:   r.Database,
	}
	query := uri.Query()
	query.Set("sslmode", sslMode)
	uri.RawQuery = query.Encode()
	db, err := sql.Open("pgx", uri.String())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}
