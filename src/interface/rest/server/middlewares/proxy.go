package middlewares

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type ProxyConfig struct {
	URL          *url.URL
	SkipPatterns [][]string
	// Rewrite defines URL path rewrite rules. The values captured in asterisk can be
	// retrieved by index e.g. $1, $2 and so on.
	// Examples:
	// "/old":              "/new",
	// "/api/*":            "/$1",
	// "/js/*":             "/public/javascripts/$1",
	// "/users/*/orders/*": "/user/$1/order/$2",
	Rewrite map[string]string
	// RegexRewrite defines rewrite rules using regexp.Rexexp with captures
	// Every capture group in the values can be retrieved by index e.g. $1, $2 and so on.
	// Example:
	// "^/old/[0.9]+/":     "/new",
	// "^/api/.+?/(.*)":    "/v2/$1",
	RegexRewrite   map[*regexp.Regexp]string
	ModifyResponse func(res *http.Response) error
	Username       string
	Password       string
}

// BasicAuth returns the encoded Basic Authentication header.
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// CustomRoundTripper is a custom implementation of http.RoundTripper to modify request headers.
type CustomRoundTripper struct {
	Transport    http.RoundTripper
	BasicAuthStr string
}

// RoundTrip executes a single HTTP transaction and adds custom headers.
func (c *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add the Basic Auth header to the request
	req.Header.Set("Authorization", c.BasicAuthStr)
	return c.Transport.RoundTrip(req)
}

// Proxy returns a proxy middleware that forwards the request to the target server.
func Proxy(config ProxyConfig) echo.MiddlewareFunc {

	// basicAuthStr := BasicAuth(config.Username, config.Password)

	return middleware.ProxyWithConfig(middleware.ProxyConfig{
		Balancer: middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
			{
				URL: config.URL,
			},
		}),
		Skipper: func(c echo.Context) bool {
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery
			for _, patterns := range config.SkipPatterns {
				isAllMatched := true
				for _, pattern := range patterns {
					if !strings.Contains(path+query, pattern) {
						isAllMatched = false
						break
					}
				}
				if isAllMatched {
					return true
				}
			}
			return false
		},
		Rewrite:        config.Rewrite,
		ModifyResponse: config.ModifyResponse,
	})
}
