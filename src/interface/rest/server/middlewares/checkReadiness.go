package middlewares

import (
	"fmt"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/go-resty/resty/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func CheckReadiness(url string, apiUser string, apiPass string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			client := resty.New()
			if apiUser != "" && apiPass != "" {
				client.SetBasicAuth(apiUser, apiPass)
			}

			// check readyz
			method := "GET"
			requestBody := clientManager.NoBody
			resReadyz := new(model.Response)

			err := clientManager.ExecuteHttpRequest(
				client,
				method,
				url,
				nil,
				clientManager.SetUseBody(requestBody),
				&requestBody,
				resReadyz,
				clientManager.VeryShortDuration,
			)

			if err != nil {
				log.Err(err).Msg("")
				return fmt.Errorf("CheckReadiness() failed: %s", err.Error())
			}

			return next(c)
		}
	}
}
