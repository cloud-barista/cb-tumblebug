package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
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
				log.Info().
					Str("id", v.RequestID).
					Str("client_ip", v.RemoteIP).
					//Str("host", v.Host).
					Str("method", v.Method).
					Str("URI", v.URI).
					//Str("user_agent", v.UserAgent).
					Int("status", v.Status).
					//Int64("latency", v.Latency.Nanoseconds()).
					Str("latency_human", v.Latency.String()).
					Str("bytes_in", v.ContentLength).
					Int64("bytes_out", v.ResponseSize).
					Msg("request")
			} else {
				log.Error().
					Err(v.Error).
					Str("id", v.RequestID).
					Str("client_ip", v.RemoteIP).
					// Str("host", v.Host).
					Str("method", v.Method).
					Str("URI", v.URI).
					//Str("user_agent", v.UserAgent).
					Int("status", v.Status).
					// Int64("latency", v.Latency.Nanoseconds()).
					Str("latency_human", v.Latency.String()).
					Str("bytes_in", v.ContentLength).
					Int64("bytes_out", v.ResponseSize).
					Msg("request error")
			}
			return nil
		},
	})
}

func RequestIdAndDetailsIssuer(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// log.Debug().Msg("Start - Request ID middleware")

		// Make X-Request-Id visible to all handlers
		c.Response().Header().Set("Access-Control-Expose-Headers", echo.HeaderXRequestID)

		// Get or generate Request ID
		reqID := c.Request().Header.Get(echo.HeaderXRequestID)
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		// Set Request on the context
		c.Set("RequestID", reqID)

		log.Trace().Msgf("(Request ID middleware) Request ID: %s", reqID)
		if _, ok := common.RequestMap.Load(reqID); ok {
			return fmt.Errorf("the X-Request-Id is already in use")
		}

		// Set "X-Request-Id" in response header
		c.Response().Header().Set(echo.HeaderXRequestID, reqID)

		details := common.RequestDetails{
			StartTime:   time.Now(),
			Status:      "Handling",
			RequestInfo: common.ExtractRequestInfo(c.Request()),
		}
		common.RequestMap.Store(reqID, details)

		// log.Debug().Msg("End - Request ID middleware")

		return next(c)
	}
}

func ResponseBodyDump() echo.MiddlewareFunc {
	return middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Skipper: func(c echo.Context) bool {
			if c.Path() == "/tumblebug/api" {
				return true
			}
			return false
		},
		Handler: func(c echo.Context, reqBody, resBody []byte) {
			// log.Debug().Msg("Start - BodyDump() middleware")

			// Get the request ID
			reqID := c.Get("RequestID").(string)
			log.Trace().Msgf("(BodyDump middleware) Request ID: %s", reqID)

			// Get the content type
			contentType := c.Response().Header().Get(echo.HeaderContentType)
			log.Trace().Msgf("contentType: %s", contentType)

			// log.Debug().Msgf("Request body: %s", string(reqBody))
			// log.Debug().Msgf("Response body: %s", string(resBody))

			// Dump the response body if content type is "application/json" or "application/json; charset=UTF-8"
			if contentType == echo.MIMEApplicationJSONCharsetUTF8 || contentType == echo.MIMEApplicationJSON {
				// Load or check the request by ID
				if v, ok := common.RequestMap.Load(reqID); ok {
					log.Trace().Msg("OK, common.RequestMap.Load(reqID)")
					details := v.(common.RequestDetails)
					details.EndTime = time.Now()

					// Set "X-Request-Id" in response header
					c.Response().Header().Set(echo.HeaderXRequestID, reqID)

					// Split the response body by newlines to handle multiple JSON objects (i.e., streaming response)
					parts := bytes.Split(resBody, []byte("\n"))
					responseJsonLines := parts[:len(parts)-1]

					// Unmarshal the latest response body
					latestResponse := responseJsonLines[len(responseJsonLines)-1]
					var resData interface{}
					if err := json.Unmarshal(latestResponse, &resData); err != nil {
						log.Error().Err(err).Msg("Error while unmarshaling response body")
						return
					}

					// Check and store error response
					// 1XX: Information responses
					// 2XX: Successful responses (200 OK, 201 Created, 202 Accepted, 204 No Content)
					// 3XX: Redirection messages
					// 4XX: Client error responses (400 Bad Request, 401 Unauthorized, 404 Not Found, 408 Request Timeout)
					// 5XX: Server error responses (500 Internal Server Error, 501 Not Implemented, 503 Service Unavailable)
					details.Status = "Success"
					if c.Response().Status >= 400 {
						details.Status = "Error"
						if data, ok := resData.(map[string]interface{}); ok {
							details.ErrorResponse = data["message"].(string)
						}
					}

					// Store the response data
					if len(responseJsonLines) > 1 {
						// handle streaming response
						// convert JSON lines to JSON array
						var responseJsonArray []interface{}
						for _, jsonLine := range responseJsonLines {
							var obj interface{}
							err := json.Unmarshal(jsonLine, &obj)
							if err != nil {
								log.Error().Err(err).Msg("error unmarshalling JSON line")
								continue
							}
							responseJsonArray = append(responseJsonArray, obj)
						}
						details.ResponseData = responseJsonArray
					} else {
						// single response
						// type casting is required
						switch data := resData.(type) {
						case map[string]interface{}:
							details.ResponseData = data
						case []interface{}:
							details.ResponseData = data
						case string:
							details.ResponseData = data
						default:
							log.Error().Msg("unexpected response data type")
						}
					}

					// Store details of the request
					common.RequestMap.Store(reqID, details)
				}
			}
			// log.Debug().Msg("Start - BodyDump() middleware")
		},
	})
}
