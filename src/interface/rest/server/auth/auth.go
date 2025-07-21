package auth

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type AuthsInfo struct {
	Authenticated bool   `json:"authenticated"`
	Role          string `json:"role"`
	Name          string `json:"name"`
	ExpiredTime   string `json:"expired-time"`
	Token         string `json:"token"`
}

// TestJWTAuth godoc
// @ID TestJWTAuth
// @Summary Test JWT authentication
// @Description Test JWT authentication
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Success 200 {object} AuthsInfo "Information of JWT authentication"
// @Failure 400 {object} object "Invalid Request"
// @Router /auth/test [get]
// @Security Bearer
func TestJWTAuth(c echo.Context) error {

	// Check if values exist and handle type assertions safely
	authVal := c.Get("authenticated")
	if authVal == nil {
		log.Error().Msg("authenticated value is nil")
		msg := model.SimpleMsg{
			Message: "authentication information not found",
		}
		return c.JSON(http.StatusUnauthorized, msg)
	}
	auth, ok := authVal.(bool)
	if !ok {
		log.Error().Msg("authenticated value is not a boolean")
		msg := model.SimpleMsg{
			Message: "invalid authentication data",
		}
		return c.JSON(http.StatusInternalServerError, msg)
	}

	tokenVal := c.Get("token")
	if tokenVal == nil {
		log.Error().Msg("token value is nil")
		msg := model.SimpleMsg{
			Message: "token information not found",
		}
		return c.JSON(http.StatusUnauthorized, msg)
	}
	token, ok := tokenVal.(string)
	if !ok {
		log.Error().Msg("token value is not a string")
		msg := model.SimpleMsg{
			Message: "invalid token data",
		}
		return c.JSON(http.StatusInternalServerError, msg)
	}

	nameVal := c.Get("name")
	if nameVal == nil {
		log.Error().Msg("name value is nil")
		msg := model.SimpleMsg{
			Message: "name information not found",
		}
		return c.JSON(http.StatusUnauthorized, msg)
	}
	name, ok := nameVal.(string)
	if !ok {
		log.Error().Msg("name value is not a string")
		msg := model.SimpleMsg{
			Message: "invalid name data",
		}
		return c.JSON(http.StatusInternalServerError, msg)
	}

	roleVal := c.Get("role")
	if roleVal == nil {
		log.Error().Msg("role value is nil")
		msg := model.SimpleMsg{
			Message: "role information not found",
		}
		return c.JSON(http.StatusUnauthorized, msg)
	}
	role, ok := roleVal.(string)
	if !ok {
		log.Error().Msg("role value is not a string")
		msg := model.SimpleMsg{
			Message: "invalid role data",
		}
		return c.JSON(http.StatusInternalServerError, msg)
	}

	expVal := c.Get("expired-time")
	if expVal == nil {
		log.Error().Msg("expired-time value is nil")
		msg := model.SimpleMsg{
			Message: "expiration information not found",
		}
		return c.JSON(http.StatusUnauthorized, msg)
	}
	exp, ok := expVal.(string)
	if !ok {
		log.Error().Msg("expired-time value is not a string")
		msg := model.SimpleMsg{
			Message: "invalid expiration data",
		}
		return c.JSON(http.StatusInternalServerError, msg)
	}

	res := &AuthsInfo{
		Authenticated: auth,
		Role:          role,
		Name:          name,
		ExpiredTime:   exp,
		Token:         token,
	}

	return c.JSON(http.StatusOK, res)
}
