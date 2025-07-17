package middlewares

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/rs/zerolog/log"
)

func Zerologger(skipPatterns [][]string) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery
			for _, patterns := range skipPatterns {
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
