package middlewares

import (
	"bytes"
	"encoding/json"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

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
			reqID := c.Request().Header.Get(echo.HeaderXRequestID)
			// log.Debug().Msgf("(BodyDump middleware) Request ID: %s", reqID)

			// Get the content type
			contentType := c.Response().Header().Get(echo.HeaderContentType)
			//log.Trace().Msgf("contentType: %s", contentType)

			// log.Debug().Msgf("Request body: %s", string(reqBody))
			// log.Debug().Msgf("Response body: %s", string(resBody))

			// Dump the response body if content type is "application/json" or "application/json; charset=UTF-8"
			if contentType == echo.MIMEApplicationJSONCharsetUTF8 || contentType == echo.MIMEApplicationJSON {
				// Load or check the request by ID
				v, ok := clientManager.RequestMap.Load(reqID)
				if !ok {
					log.Error().Msg("Request ID not found in common.RequestMap")
					return
				}

				// Ensure the loaded value is of the correct type
				details, ok := v.(clientManager.RequestDetails)
				if !ok {
					log.Error().Msg("Loaded value from common.RequestMap is not of type clientManager.RequestDetails")
					return
				}
				//log.Trace().Msg("OK, common.RequestMap.Load(reqID)")
				details.EndTime = time.Now()

				// Set "X-Request-Id" in response header
				c.Response().Header().Set(echo.HeaderXRequestID, reqID)

				// Split the response body by newlines to handle multiple JSON objects (i.e., streaming response)
				parts := bytes.Split(resBody, []byte("\n"))
				if len(parts) == 0 {
					log.Error().Msg("Response body is empty")
					return
				}
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
						log.Error().Msgf("unexpected response data type (%T)", data)
					}
				}

				// Store details of the request
				clientManager.RequestMap.Store(reqID, details)
			}
			// log.Debug().Msg("Start - BodyDump() middleware")
		},
	})
}
