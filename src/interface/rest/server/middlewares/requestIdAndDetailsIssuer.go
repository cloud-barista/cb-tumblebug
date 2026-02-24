package middlewares

import (
	"fmt"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/labstack/echo/v4"
)

func RequestIdAndDetailsIssuer(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// log.Debug().Msg("Start - Request ID middleware")

		// Make X-Request-Id visible to all handlers
		c.Response().Header().Set("Access-Control-Expose-Headers", echo.HeaderXRequestID)

		// Get or generate Request ID
		reqID := c.Request().Header.Get(echo.HeaderXRequestID)
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
			// Set the generated request ID to the request header
			c.Request().Header.Set(echo.HeaderXRequestID, reqID)
		}

		//log.Trace().Msgf("(Request ID middleware) Request ID: %s", reqID)
		if _, ok := clientManager.RequestMap.Load(reqID); ok {
			return fmt.Errorf("the X-Request-Id is already in use")
		}

		// Set "X-Request-Id" in response header
		c.Response().Header().Set(echo.HeaderXRequestID, reqID)

		details := clientManager.RequestDetails{
			StartTime:   time.Now(),
			Status:      "Handling",
			RequestInfo: clientManager.ExtractRequestInfo(c.Request()),
		}
		clientManager.RequestMap.Store(reqID, details)
		// Note: requestMapEntryCount is an approximate counter and may temporarily
		// diverge from the actual number of entries in RequestMap.
		clientManager.IncrementRequestMapCount()

		// log.Debug().Msg("End - Request ID middleware")

		return next(c)
	}
}
