package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite：%w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 SQLite：%w", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(type, name)
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			access_key TEXT NOT NULL,
			secret_ref TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'unknown',
			last_tested_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(provider_id) REFERENCES providers(id) ON DELETE RESTRICT,
			UNIQUE(provider_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS buckets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			region TEXT,
			endpoint TEXT,
			cdn_domain TEXT,
			private INTEGER NOT NULL DEFAULT 0,
			selected INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			UNIQUE(account_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			remote_prefix TEXT NOT NULL,
			local_path TEXT NOT NULL,
			version_file TEXT NOT NULL DEFAULT 'version.json',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(bucket_id) REFERENCES buckets(id) ON DELETE CASCADE,
			UNIQUE(bucket_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS cdn_bindings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL,
			provider_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			domain TEXT NOT NULL,
			credential_ref TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE,
			FOREIGN KEY(provider_id) REFERENCES providers(id) ON DELETE RESTRICT,
			UNIQUE(site_id, domain)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL,
			local_version TEXT,
			remote_version TEXT,
			upload_mode TEXT NOT NULL,
			refresh_cdn INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			message TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
		)`,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始数据库迁移事务：%w", err)
	}
	defer tx.Rollback()

	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行数据库迁移：%w", err)
		}
	}

	if err := ensureBucketCDNColumn(ctx, tx); err != nil {
		return err
	}
	if err := ensureBucketSelectedColumn(ctx, tx); err != nil {
		return err
	}
	if err := ensureBucketPrivateColumn(ctx, tx); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`); err != nil {
		return fmt.Errorf("记录数据库版本：%w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交数据库迁移：%w", err)
	}

	return nil
}

func ensureBucketPrivateColumn(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(buckets)`)
	if err != nil {
		return fmt.Errorf("读取 buckets 表结构：%w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("解析 buckets 表结构：%w", err)
		}
		if name == "private" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("读取 buckets 表结构结果：%w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE buckets ADD COLUMN private INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("添加 Bucket 访问属性字段：%w", err)
	}
	return nil
}

func ensureBucketSelectedColumn(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(buckets)`)
	if err != nil {
		return fmt.Errorf("读取 buckets 表结构：%w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("解析 buckets 表结构：%w", err)
		}
		if name == "selected" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("读取 buckets 表结构结果：%w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE buckets ADD COLUMN selected INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("添加 Bucket 选择字段：%w", err)
	}
	return nil
}

func ensureBucketCDNColumn(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(buckets)`)
	if err != nil {
		return fmt.Errorf("读取 buckets 表结构：%w", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("解析 buckets 表结构：%w", err)
		}
		if name == "cdn_domain" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("读取 buckets 表结构结果：%w", err)
	}
	if found {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE buckets ADD COLUMN cdn_domain TEXT`); err != nil {
		return fmt.Errorf("添加 Bucket CDN 域名字段：%w", err)
	}
	return nil
}
