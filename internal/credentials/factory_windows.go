//go:build windows

package credentials

import "fmt"

func NewDefaultStore(path string) (Store, error) {
	backend := NewDPAPIStore()
	store, err := NewFileStore(path, backend)
	if err != nil {
		return nil, fmt.Errorf("创建 Windows 凭据存储：%w", err)
	}

	return store, nil
}
