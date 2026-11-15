//go:build !windows

package updater

import (
	"net/http"
	"net/url"
)

func proxyFromSystemOrEnvironment() func(*http.Request) (*url.URL, error) {
	return http.ProxyFromEnvironment
}
