//go:build !windows

package credentials

import (
	"context"
	"fmt"
)

type DPAPIStore struct{}

func NewDPAPIStore() *DPAPIStore {
	return &DPAPIStore{}
}

func (s *DPAPIStore) Save(context.Context, string, []byte) error {
	return fmt.Errorf("当前平台不支持 Windows DPAPI")
}

func (s *DPAPIStore) Load(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("当前平台不支持 Windows DPAPI")
}

func (s *DPAPIStore) Delete(context.Context, string) error {
	return fmt.Errorf("当前平台不支持 Windows DPAPI")
}

func (s *DPAPIStore) Exists(context.Context, string) (bool, error) {
	return false, fmt.Errorf("当前平台不支持 Windows DPAPI")
}

func (s *DPAPIStore) Seal(string, []byte) ([]byte, error) {
	return nil, fmt.Errorf("当前平台不支持 Windows DPAPI")
}

func (s *DPAPIStore) Open(string, []byte) ([]byte, error) {
	return nil, fmt.Errorf("当前平台不支持 Windows DPAPI")
}
