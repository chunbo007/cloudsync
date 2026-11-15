//go:build windows

package updater

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func proxyFromSystemOrEnvironment() func(*http.Request) (*url.URL, error) {
	systemProxy, overrides := readWindowsProxy()
	environmentProxy := http.ProxyFromEnvironment
	return func(request *http.Request) (*url.URL, error) {
		if matchesProxyOverride(request.URL.Hostname(), overrides) {
			return nil, nil
		}
		if systemProxy != nil {
			return systemProxy, nil
		}
		return environmentProxy(request)
	}
}

func readWindowsProxy() (*url.URL, []string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil, nil
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil, nil
	}
	proxyServer, _, err := key.GetStringValue("ProxyServer")
	if err != nil || strings.TrimSpace(proxyServer) == "" {
		return nil, nil
	}
	override, _, _ := key.GetStringValue("ProxyOverride")
	proxy, err := parseWindowsProxy(proxyServer, "https")
	if err != nil {
		return nil, strings.Split(override, ";")
	}
	return proxy, strings.Split(override, ";")
}

func parseWindowsProxy(value, scheme string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	for _, part := range strings.Split(value, ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(pair[0]), scheme) {
			value = strings.TrimSpace(pair[1])
			break
		}
	}
	if strings.Contains(value, "=") {
		for _, part := range strings.Split(value, ";") {
			pair := strings.SplitN(part, "=", 2)
			if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(pair[0]), "http") {
				value = strings.TrimSpace(pair[1])
				break
			}
		}
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	return url.Parse(value)
}

func matchesProxyOverride(host string, overrides []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, item := range overrides {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" || item == "<local>" {
			if item == "<local>" && !strings.Contains(host, ".") {
				return true
			}
			continue
		}
		item = strings.TrimPrefix(item, "*")
		if strings.HasSuffix(host, item) || host == item {
			return true
		}
	}
	return false
}
