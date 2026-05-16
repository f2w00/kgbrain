package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open 打开/创建 SQLite 数据库并执行 PRAGMA 配置.
// pragmas 由外部传入, 支持自定义配置.
// 返回的 *sql.DB 可跨多个 repository 共享.
func Open(path string, pragmas []string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	return db, nil
}
