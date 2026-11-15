//go:build !windows

package credentials

import "fmt"

func NewDefaultStore(string) (Store, error) {
	return nil, fmt.Errorf("当前平台暂未配置凭据存储实现")
}
