//go:build windows

package credentials

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStorePersistsDPAPISecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	store, err := NewDefaultStore(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := store.Save(ctx, "account/qiniu/test", []byte("secret-value")); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewDefaultStore(path)
	if err != nil {
		t.Fatal(err)
	}

	value, err := reloaded.Load(ctx, "account/qiniu/test")
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "secret-value" {
		t.Fatalf("解密结果不正确：%q", value)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "secret-value" {
		t.Fatal("凭据文件不应保存明文 SecretKey")
	}
}
