package database

import (
	"context"
	"fmt"
)

type Bucket struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"accountId"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	Private   bool   `json:"private"`
	Selected  bool   `json:"selected"`
}

func (db *DB) SaveBucketSelections(ctx context.Context, accountID int64, buckets []Bucket) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE buckets SET selected = 0 WHERE account_id = ?`, accountID); err != nil {
		return fmt.Errorf("清理 Bucket 选择：%w", err)
	}
	for _, bucket := range buckets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO buckets(account_id, name, region, private, selected)
			VALUES (?, ?, ?, ?, 1)
			ON CONFLICT(account_id, name) DO UPDATE SET
				region = excluded.region,
				private = excluded.private,
				selected = 1,
				updated_at = CURRENT_TIMESTAMP
			`, accountID, bucket.Name, bucket.Region, boolToInt(bucket.Private)); err != nil {
			return fmt.Errorf("保存 Bucket 选择：%w", err)
		}
	}
	return tx.Commit()
}

func (db *DB) ListBuckets(ctx context.Context, accountID int64, selectedOnly bool) ([]Bucket, error) {
	query := `
		SELECT id, account_id, name, COALESCE(region, 'auto'), private, selected
		FROM buckets WHERE account_id = ?`
	if selectedOnly {
		query += ` AND selected = 1`
	}
	query += ` ORDER BY name`

	rows, err := db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("读取 Bucket：%w", err)
	}
	defer rows.Close()

	result := make([]Bucket, 0)
	for rows.Next() {
		var bucket Bucket
		var private, selected int
		if err := rows.Scan(&bucket.ID, &bucket.AccountID, &bucket.Name, &bucket.Region, &private, &selected); err != nil {
			return nil, fmt.Errorf("解析 Bucket：%w", err)
		}
		bucket.Selected = selected == 1
		bucket.Private = private == 1
		result = append(result, bucket)
	}
	return result, rows.Err()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (db *DB) DeleteBucket(ctx context.Context, accountID int64, name string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM buckets WHERE account_id = ? AND name = ?`, accountID, name)
	if err != nil {
		return fmt.Errorf("删除本地 Bucket 配置：%w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("确认本地 Bucket 配置删除结果：%w", err)
	}
	if count == 0 {
		return fmt.Errorf("本地 Bucket 配置不存在")
	}
	return nil
}
