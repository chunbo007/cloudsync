package database

import (
	"context"
	"fmt"
)

type Site struct {
	ID           int64  `json:"id"`
	AccountID    int64  `json:"accountId"`
	BucketID     int64  `json:"bucketId"`
	Name         string `json:"name"`
	RemotePrefix string `json:"remotePrefix"`
	LocalPath    string `json:"localPath"`
	VersionFile  string `json:"versionFile"`
	AccountName  string `json:"accountName,omitempty"`
	BucketName   string `json:"bucketName,omitempty"`
	Private      bool   `json:"private,omitempty"`
}

func (db *DB) ListAllSites(ctx context.Context) ([]Site, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, b.account_id, s.bucket_id, s.name, s.remote_prefix, s.local_path, s.version_file,
		       a.name, b.name, b.private
		FROM sites s
		JOIN buckets b ON b.id = s.bucket_id
		JOIN accounts a ON a.id = b.account_id
		ORDER BY a.name, b.name, s.name
	`)
	if err != nil {
		return nil, fmt.Errorf("读取全部路径映射：%w", err)
	}
	defer rows.Close()
	result := make([]Site, 0)
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.AccountID, &site.BucketID, &site.Name, &site.RemotePrefix, &site.LocalPath, &site.VersionFile, &site.AccountName, &site.BucketName, &site.Private); err != nil {
			return nil, fmt.Errorf("解析全部路径映射：%w", err)
		}
		result = append(result, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取全部路径映射结果：%w", err)
	}
	return result, nil
}

func (db *DB) SaveSite(ctx context.Context, site Site) (int64, error) {
	if site.AccountID == 0 || site.Name == "" || site.LocalPath == "" {
		return 0, fmt.Errorf("账号、本地目录和网站名称不能为空")
	}
	if site.VersionFile == "" {
		site.VersionFile = "version.json"
	}

	var bucketID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM buckets WHERE account_id = ? LIMIT 1`, site.AccountID).Scan(&bucketID); err != nil {
		return 0, fmt.Errorf("读取账号 Bucket：%w", err)
	}

	var siteID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO sites(bucket_id, name, remote_prefix, local_path, version_file)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, name) DO UPDATE SET
			remote_prefix = excluded.remote_prefix,
			local_path = excluded.local_path,
			version_file = excluded.version_file,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, bucketID, site.Name, site.RemotePrefix, site.LocalPath, site.VersionFile).Scan(&siteID)
	if err != nil {
		return 0, fmt.Errorf("保存网站：%w", err)
	}
	return siteID, nil
}

func (db *DB) SaveSiteForBucket(ctx context.Context, accountID int64, bucketName string, site Site) (int64, error) {
	if accountID == 0 || bucketName == "" || site.Name == "" || site.LocalPath == "" {
		return 0, fmt.Errorf("账号、Bucket、本地目录和映射名称不能为空")
	}
	if site.VersionFile == "" {
		site.VersionFile = ""
	}
	var bucketID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM buckets WHERE account_id = ? AND name = ?`, accountID, bucketName).Scan(&bucketID); err != nil {
		return 0, fmt.Errorf("读取目标 Bucket：%w", err)
	}
	var siteID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO sites(bucket_id, name, remote_prefix, local_path, version_file)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, name) DO UPDATE SET
			remote_prefix = excluded.remote_prefix,
			local_path = excluded.local_path,
			version_file = excluded.version_file,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, bucketID, site.Name, site.RemotePrefix, site.LocalPath, site.VersionFile).Scan(&siteID)
	if err != nil {
		return 0, fmt.Errorf("保存路径映射：%w", err)
	}
	return siteID, nil
}

func (db *DB) ListSites(ctx context.Context, accountID int64) ([]Site, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, b.account_id, s.bucket_id, s.name, s.remote_prefix, s.local_path, s.version_file
		FROM sites s
		JOIN buckets b ON b.id = s.bucket_id
		WHERE b.account_id = ?
		ORDER BY s.name
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("读取网站：%w", err)
	}
	defer rows.Close()

	sites := make([]Site, 0)
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.AccountID, &site.BucketID, &site.Name, &site.RemotePrefix, &site.LocalPath, &site.VersionFile); err != nil {
			return nil, fmt.Errorf("解析网站：%w", err)
		}
		sites = append(sites, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取网站结果：%w", err)
	}
	return sites, nil
}

func (db *DB) ListSitesForBucket(ctx context.Context, accountID int64, bucketName string) ([]Site, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, b.account_id, s.bucket_id, s.name, s.remote_prefix, s.local_path, s.version_file
		FROM sites s
		JOIN buckets b ON b.id = s.bucket_id
		WHERE b.account_id = ? AND b.name = ?
		ORDER BY s.name
	`, accountID, bucketName)
	if err != nil {
		return nil, fmt.Errorf("读取路径映射：%w", err)
	}
	defer rows.Close()
	result := make([]Site, 0)
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.AccountID, &site.BucketID, &site.Name, &site.RemotePrefix, &site.LocalPath, &site.VersionFile); err != nil {
			return nil, fmt.Errorf("解析路径映射：%w", err)
		}
		result = append(result, site)
	}
	return result, rows.Err()
}

func (db *DB) DeleteSite(ctx context.Context, siteID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, siteID)
	if err != nil {
		return fmt.Errorf("删除路径映射：%w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("路径映射不存在")
	}
	return nil
}

func (db *DB) UpdateSite(ctx context.Context, siteID int64, name, localPath, remotePrefix string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE sites
		SET name = ?, local_path = ?, remote_prefix = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, localPath, remotePrefix, siteID)
	if err != nil {
		return fmt.Errorf("更新路径映射：%w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("路径映射不存在")
	}
	return nil
}

func (db *DB) GetSite(ctx context.Context, siteID int64) (Site, error) {
	var site Site
	err := db.QueryRowContext(ctx, `
		SELECT s.id, b.account_id, s.bucket_id, s.name, s.remote_prefix, s.local_path, s.version_file
		FROM sites s
		JOIN buckets b ON b.id = s.bucket_id
		WHERE s.id = ?
	`, siteID).Scan(
		&site.ID,
		&site.AccountID,
		&site.BucketID,
		&site.Name,
		&site.RemotePrefix,
		&site.LocalPath,
		&site.VersionFile,
	)
	if err != nil {
		return Site{}, fmt.Errorf("读取网站：%w", err)
	}
	return site, nil
}
