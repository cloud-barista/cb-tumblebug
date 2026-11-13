package middlewares

import (
	"net/http"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/logfilter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/rs/zerolog/log"
)

func Zerologger(skipRules []logfilter.SkipRule) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			url := c.Request().URL.Path
			if q := c.Request().URL.RawQuery; q != "" {
				url += "?" + q
			}
			return logfilter.ShouldSkip(skipRules, c.Request().Method, url)
		},
		// Logged before the handler runs so long-running requests are visible while in flight
		BeforeNextFunc: func(c echo.Context) {
			if c.Request().Method != http.MethodOptions {
				log.Info().
					Str("Method", c.Request().Method).
					Str("URI", c.Request().RequestURI).
					Str("clientIP", c.RealIP()).
					Msg("request start")
			}
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

// SlowRequestLogger logs requests that the request logger skips (high-frequency polling GETs) once they take
// threshold or longer, so a stalled poll still leaves a trace. The latency includes writing the response.
func SlowRequestLogger(skipRules []logfilter.SkipRule, threshold time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if d := time.Since(start); d >= threshold {
				url := c.Request().URL.Path
				if q := c.Request().URL.RawQuery; q != "" {
					url += "?" + q
				}
				if logfilter.ShouldSkip(skipRules, c.Request().Method, url) {
					log.Warn().
						Str("Method", c.Request().Method).
						Str("URI", url).
						Str("clientIP", c.RealIP()).
						Int("status", c.Response().Status).
						Int64("bytesOut", c.Response().Size).
						Str("latency", d.String()).
						Msg("slow request")
				}
			}
			return err
		}
	}
}
