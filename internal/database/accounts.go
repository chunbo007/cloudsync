package database

import (
	"context"
	"fmt"
)

type Account struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	AccessKey   string `json:"accessKey"`
	SecretRef   string `json:"secretRef"`
	Bucket      string `json:"bucket"`
	BucketCount int    `json:"bucketCount"`
	Region      string `json:"region"`
	CDNDomain   string `json:"cdnDomain"`
	Status      string `json:"status"`
}

func (db *DB) SaveQiniuAccount(ctx context.Context, account Account) (int64, error) {
	if account.Name == "" || account.AccessKey == "" || account.SecretRef == "" {
		return 0, fmt.Errorf("账号名称、AccessKey 和 SecretRef 不能为空")
	}
	if account.Provider == "" {
		account.Provider = "qiniu"
	}
	if account.Region == "" {
		account.Region = "auto"
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var providerID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO providers(type, name)
		VALUES (?, ?)
		ON CONFLICT(type, name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, account.Provider, account.Provider).Scan(&providerID)
	if err != nil {
		return 0, fmt.Errorf("保存云平台：%w", err)
	}

	var accountID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO accounts(provider_id, name, access_key, secret_ref, status)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(provider_id, name) DO UPDATE SET
			access_key = excluded.access_key,
			secret_ref = excluded.secret_ref,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, providerID, account.Name, account.AccessKey, account.SecretRef, "configured").Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("保存账号：%w", err)
	}

	if account.Bucket == "" {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("提交账号配置：%w", err)
		}
		return accountID, nil
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO buckets(account_id, name, region, endpoint, cdn_domain)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_id, name) DO UPDATE SET
			region = excluded.region,
			cdn_domain = excluded.cdn_domain,
			updated_at = CURRENT_TIMESTAMP
	`, accountID, account.Bucket, account.Region, "", account.CDNDomain)
	if err != nil {
		return 0, fmt.Errorf("保存 Bucket：%w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交账号配置：%w", err)
	}
	return accountID, nil
}

func (db *DB) ListQiniuAccounts(ctx context.Context) ([]Account, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, p.type, a.name, a.access_key, a.secret_ref,
		       COALESCE((SELECT COUNT(*) FROM buckets bx WHERE bx.account_id = a.id), 0),
		       a.status
		FROM accounts a
		JOIN providers p ON p.id = a.provider_id
		ORDER BY a.name
	`)
	if err != nil {
		return nil, fmt.Errorf("读取账号：%w", err)
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		var account Account
		if err := rows.Scan(
			&account.ID,
			&account.Provider,
			&account.Name,
			&account.AccessKey,
			&account.SecretRef,
			&account.BucketCount,
			&account.Status,
		); err != nil {
			return nil, fmt.Errorf("解析账号：%w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取账号结果：%w", err)
	}
	return accounts, nil
}

func (db *DB) GetQiniuAccount(ctx context.Context, id int64) (Account, error) {
	var account Account
	err := db.QueryRowContext(ctx, `
		SELECT a.id, p.type, a.name, a.access_key, a.secret_ref,
		       COALESCE(b.name, ''), COALESCE(b.region, 'auto'), COALESCE(b.cdn_domain, ''), a.status
		FROM accounts a
		JOIN providers p ON p.id = a.provider_id
		LEFT JOIN buckets b ON b.account_id = a.id
		WHERE a.id = ?
	`, id).Scan(
		&account.ID,
		&account.Provider,
		&account.Name,
		&account.AccessKey,
		&account.SecretRef,
		&account.Bucket,
		&account.Region,
		&account.CDNDomain,
		&account.Status,
	)
	if err != nil {
		return Account{}, fmt.Errorf("读取账号：%w", err)
	}
	return account, nil
}

func (db *DB) DeleteQiniuAccount(ctx context.Context, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除账号：%w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("确认账号删除结果：%w", err)
	}
	if count == 0 {
		return fmt.Errorf("账号不存在")
	}
	return nil
}

func (db *DB) UpdateQiniuAccount(ctx context.Context, id int64, name, accessKey, secretRef string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE accounts
		SET name = ?, access_key = ?, secret_ref = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, accessKey, secretRef, id)
	if err != nil {
		return fmt.Errorf("更新账号：%w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("账号不存在")
	}
	return nil
}
