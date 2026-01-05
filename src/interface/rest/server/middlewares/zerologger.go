package middlewares

import (
	"net/http"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/logfilter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/rs/zerolog/log"
)

func Zerologger(skipRules []logfilter.SkipRule) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery
			method := c.Request().Method

			// Build URL with proper separator
			url := path
			if query != "" {
				url = path + "?" + query
			}

			for _, rule := range skipRules {
				// Check method filter (empty = match any)
				if rule.Method != "" && rule.Method != method {
					continue
				}

				// Check all URL patterns (AND condition)
				allMatched := true
				for _, pattern := range rule.Patterns {
					if !strings.Contains(url, pattern) {
						allMatched = false
						break
					}
				}
				if allMatched {
					return true
				}
			}
			return false
		},
		LogError:         true,
		LogRequestID:     true,
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogUserAgent:     false,
		LogStatus:        true,
		LogLatency:       true,
		LogContentLength: true,
		LogResponseSize:  true,
		// HandleError:      true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				if v.Method != http.MethodOptions {
					log.Info().
						Str("ID", v.RequestID).
						Str("Method", v.Method).
						Str("URI", v.URI).
						Str("clientIP", v.RemoteIP).
						//Str("host", v.Host).
						//Str("user_agent", v.UserAgent).
						Int("status", v.Status).
						//Int64("latency", v.Latency.Nanoseconds()).
						Str("latency", v.Latency.String()).
						//Str("bytes_in", v.ContentLength).
						//Int64("bytes_out", v.ResponseSize).
						Msg("request")
				}
			} else {
				log.Error().
					Err(v.Error).
					Str("ID", v.RequestID).
					Str("Method", v.Method).
					Str("URI", v.URI).
					Str("clientIP", v.RemoteIP).
					// Str("host", v.Host).
					//Str("user_agent", v.UserAgent).
					Int("status", v.Status).
					// Int64("latency", v.Latency.Nanoseconds()).
					Str("latency", v.Latency.String()).
					//Str("bytes_in", v.ContentLength).
					//Int64("bytes_out", v.ResponseSize).
					Msg("request error")
			}
			return nil
		},
	})
}
