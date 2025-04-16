package authmw

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/m-cmp/mc-iam-manager/iamtokenvalidator"
	"github.com/rs/zerolog/log"
)

func InitJwtAuthMw(iamEndpoint string, pubkeyUrl string) error {
	log.Debug().Msg("Start - InitJwtAuthMw")

	// Check readiness of MC-IAM-Manager
	client := resty.New()

	method := "GET"
	url := fmt.Sprintf("%s/readyz", iamEndpoint)
	requestBody := clientManager.NoBody
	var resReadyz map[string]string

	err := clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&resReadyz,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		return err
	}

	// log.Debug().Msgf("resReadyz: %+v", resReadyz["status"])
	// if resReadyz["status"] != "ok" {
	// 	return fmt.Errorf("MC-IAM-Manager is not ready")
	// }

	// Get a public key from MC-IAM-Manager
	err = iamtokenvalidator.GetPubkeyIamManager(iamEndpoint + pubkeyUrl)
	if err != nil {
		log.Debug().Msgf("failed to get public key from IAM Manager: %v", err.Error())
	}

	log.Debug().Msg("End - InitJwtAuthMw")
	return nil
}

// JwtAuthMw initializes and returns the JWT middleware.
func JwtAuthMw(skipPatterns [][]string) echo.MiddlewareFunc {

	log.Debug().Msg("Start - JWTAuthMW")

	config := echojwt.Config{
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
		// SigningMethod:  signingMethod,
		KeyFunc:        iamtokenvalidator.Keyfunction,
		SuccessHandler: retrospectToken,
	}
	log.Debug().Msg("End - JWTAuthMW")

	return echojwt.WithConfig(config)
}

// The SuccessHandler for the JWT middleware
// It will be called if jwt.Parse succeeds and set the claims in the context.
// (Briefly, it is the process of checking whether a (previously) issued token is still valid or not.)
func retrospectToken(c echo.Context) {
	log.Debug().Msg("start - retrospectToken, which is the SuccessHandler")

	accesstoken := c.Get("user").(*jwt.Token).Raw
	iamManagerClaims, err := iamtokenvalidator.GetTokenClaimsByIamManagerClaims(accesstoken)
	if err != nil {
		c.String(http.StatusUnauthorized, "failed to type cast claims as jwt.MapClaims")
	}

	jwtRegisteredClaims := iamManagerClaims.RegisteredClaims
	log.Debug().Msgf("claims.RegisteredClaims: %+v", jwtRegisteredClaims)

	// Get the realm roles from the claims
	roles := iamManagerClaims.RealmAccess.Roles
	log.Debug().Msgf("claims.RealmAccess.Roles: %+v", roles)

	// Check this user's role
	var role = ""
	if HasRole(roles, "maintainer") {
		role = "maintainer"
	} else if HasRole(roles, "admin") {
		role = "admin"
	} else if HasRole(roles, "user") {
		role = "user"
	} else {
		role = "guest"
	}

	// Get expiry time from claims
	exp := jwtRegisteredClaims.ExpiresAt
	log.Debug().Msgf("claims.ExpiresAt: %+v", exp)

	expiryTime := time.Unix(exp.Unix(), 0)         // Unix time
	expiredTime := expiryTime.Format(time.RFC3339) // RFC3339 time
	log.Debug().Msgf("expiredTime: %+v", expiredTime)

	// log.Trace().Msgf("token: %+v", token)
	log.Trace().Msgf("accesstoken (jwtToken.Raw): %+v", accesstoken)
	log.Trace().Msgf("claims: %+v", iamManagerClaims)

	// Set user as authenticated
	c.Set("authenticated", true)
	c.Set("token", accesstoken)
	// Set user name
	c.Set("name", iamManagerClaims.Name)
	c.Set("role", role)
	c.Set("expired-time", expiredTime)
	// Set more values here
	// ...

	log.Debug().Msg("End - retrospectToken, which is the SuccessHandler")
}

// HasRole checks if a slice contains a specific element
func HasRole(roleList []string, role string) bool {
	for _, s := range roleList {
		if s == role {
			return true
		}
	}
	return false
}

// [Keep this code block] This function is required for frontend web server
// func SessionCheckerMW(next echo.HandlerFunc) echo.HandlerFunc {
// 	return func(c echo.Context) error {
// 		sess, err := session.Get("session", c)
// 		if err != nil {
// 			log.Error().Err(err).Msg("failed to get session")
// 			return c.Redirect(http.StatusSeeOther, "/")
// 		}

// 		expiredTime, ok := sess.Values["expired-time"].(string)
// 		if !ok {
// 			log.Error().Msg("failed to cast sess.Values[expired-time] as string")
// 			// Delete session if it's expired
// 			sess.Options.MaxAge = -1 //
// 			sess.Save(c.Request(), c.Response())
// 			return c.Redirect(http.StatusSeeOther, "/")
// 		}
// 		log.Trace().Msgf("sess.Values[expired-time] %v", expiredTime)

// 		expires, err := time.Parse(time.RFC3339, expiredTime)
// 		if err != nil {
// 			log.Error().Err(err).Msg("failed to parse expiredTime")
// 			// Delete session if it's expired
// 			sess.Options.MaxAge = -1 //
// 			sess.Save(c.Request(), c.Response())
// 			return c.Redirect(http.StatusSeeOther, "/")
// 		}

// 		if time.Now().After(expires) {
// 			log.Error().Msg("session expired")
// 			// Delete session if it's expired
// 			sess.Options.MaxAge = -1 //
// 			sess.Save(c.Request(), c.Response())
// 			return c.Redirect(http.StatusSeeOther, "/")
// 		}

// 		log.Trace().Msgf("sess.Values[authenticated]: %v", sess.Values["authenticated"])
// 		log.Trace().Msgf("sess.Values[token]: %v", sess.Values["token"])
// 		log.Trace().Msgf("sess.Values[name]: %v", sess.Values["name"])
// 		log.Trace().Msgf("sess.Values[role]: %v", sess.Values["role"])

// 		return next(c)
// 	}
// }
