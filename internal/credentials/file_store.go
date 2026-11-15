package credentials

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FileStore struct {
	mu      sync.Mutex
	path    string
	backend Cipher
	values  map[string]string
}

func NewFileStore(path string, backend Cipher) (*FileStore, error) {
	store := &FileStore{
		path:    path,
		backend: backend,
		values:  make(map[string]string),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *FileStore) Save(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	ciphertext, err := s.backend.Seal(key, value)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.values[key] = base64.StdEncoding.EncodeToString(ciphertext)
	err = s.persistLocked()
	s.mu.Unlock()
	return err
}

func (s *FileStore) Load(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	value, ok := s.values[key]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("凭据引用不存在：%s", key)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("解码凭据：%w", err)
	}

	return s.backend.Open(key, ciphertext)
}

func (s *FileStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.values, key)
	err := s.persistLocked()
	s.mu.Unlock()
	return err
}

func (s *FileStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.Lock()
	_, ok := s.values[key]
	s.mu.Unlock()
	return ok, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取凭据文件：%w", err)
	}

	if err := json.Unmarshal(data, &s.values); err != nil {
		return fmt.Errorf("解析凭据文件：%w", err)
	}

	return nil
}

func (s *FileStore) persistLocked() error {
	data, err := json.MarshalIndent(s.values, "", "  ")
	if err != nil {
		return fmt.Errorf("编码凭据文件：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("创建凭据目录：%w", err)
	}

	temporaryPath := s.path + ".tmp"
	if err := os.WriteFile(temporaryPath, data, 0o600); err != nil {
		return fmt.Errorf("写入凭据文件：%w", err)
	}

	if err := os.Rename(temporaryPath, s.path); err != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("替换凭据文件：%w", err)
	}

	return nil
}
