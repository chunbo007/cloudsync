//go:build windows

package credentials

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type DPAPIStore struct {
	mu     sync.RWMutex
	values map[string][]byte
}

func NewDPAPIStore() *DPAPIStore {
	return &DPAPIStore{values: make(map[string][]byte)}
}

func (s *DPAPIStore) Save(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	encrypted, err := s.Seal(key, value)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.values[key] = encrypted
	s.mu.Unlock()
	return nil
}

func (s *DPAPIStore) Load(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	encrypted, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("凭据不存在：%s", key)
	}

	return s.Open(key, encrypted)
}

func (s *DPAPIStore) Seal(key string, value []byte) ([]byte, error) {
	return protect(value, key)
}

func (s *DPAPIStore) Open(key string, value []byte) ([]byte, error) {
	return unprotect(value, key)
}

func (s *DPAPIStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
	return nil
}

func (s *DPAPIStore) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.RLock()
	_, ok := s.values[key]
	s.mu.RUnlock()
	return ok, nil
}

func protect(value []byte, key string) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(value))}
	if len(value) > 0 {
		input.Data = &value[0]
	}

	entropy := entropyFor(key)
	optionalEntropy := windows.DataBlob{Size: uint32(len(entropy))}
	optionalEntropy.Data = &entropy[0]

	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, &optionalEntropy, 0, nil, 0, &output); err != nil {
		return nil, fmt.Errorf("DPAPI 加密失败：%w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))

	result := append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	return result, nil
}

func unprotect(value []byte, key string) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(value))}
	if len(value) > 0 {
		input.Data = &value[0]
	}

	entropy := entropyFor(key)
	optionalEntropy := windows.DataBlob{Size: uint32(len(entropy))}
	optionalEntropy.Data = &entropy[0]

	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, &optionalEntropy, 0, nil, 0, &output); err != nil {
		return nil, fmt.Errorf("DPAPI 解密失败：%w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))

	result := append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	return result, nil
}

func entropyFor(key string) []byte {
	digest := sha256.Sum256([]byte("qiniu-site-manager:" + key))
	return digest[:]
}
