package ffuf

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type ProxyFunc func(*http.Request) (*url.URL, error)

type ProxyPool struct {
	Proxies []ProxyFunc

	index       int
	accessMutex sync.Mutex
}

func NewProxyPool(filePath string) (*ProxyPool, error) {
	pool := ProxyPool{}

	{
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("Error opening proxy file: %w", err)
		}

		proxies := []ProxyFunc{}

		scanner := bufio.NewScanner(f)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			} else if _, err := url.Parse(line); err != nil {
				continue
			}

			if strings.HasPrefix(line, "#") {
				continue
			}

			u, isValid := parseProxyUrl(line)
			if !isValid {
				return nil, fmt.Errorf("Bad proxy url (-x) format. Expected http, https or socks5 url")
			}

			proxies = append(proxies, http.ProxyURL(u))
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("Error reading proxy file: %w", err)
		}

		pool.Proxies = proxies
	}

	return &pool, nil
}

// Request a proxy from the pool
func (pool *ProxyPool) Get() (ProxyFunc, error) {
	pool.accessMutex.Lock()
	defer pool.accessMutex.Unlock()

	poolLen := len(pool.Proxies)
	if poolLen == 0 {
		return nil, fmt.Errorf("No proxies available in the pool")
	}

	proxy := pool.Proxies[pool.index]
	if pool.index == poolLen-1 {
		pool.index = 0
	} else {
		pool.index++
	}

	return proxy, nil
}
