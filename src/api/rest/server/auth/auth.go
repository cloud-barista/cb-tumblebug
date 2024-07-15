package auth

import (
	"net/http"

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
// @Summary Test JWT authentication
// @Description Test JWT authentication
// @Tags [Auth] Test JWT authentication
// @Accept  json
// @Produce  json
// @Success 200 {object} AuthsInfo "Information of JWT authentication"
// @Failure 400 {object} object "Invalid Request"
// @Router /auth/test [get]
// @Security Bearer
func TestJWTAuth(c echo.Context) error {

	auth := c.Get("authenticated").(bool)
	token := c.Get("token").(string)
	name := c.Get("name").(string)
	role := c.Get("role").(string)
	exp := c.Get("expired-time").(string)

	log.Debug().
		Bool("authenticated", auth).
		Str("token", token).
		Str("name", name).
		Str("role", role).
		Str("expired-time", exp).
		Msg("TestJWTAuth")

	res := &AuthsInfo{
		Authenticated: auth,
		Role:          role,
		Name:          name,
		ExpiredTime:   exp,
		Token:         token,
	}

	return c.JSON(http.StatusOK, res)
}
