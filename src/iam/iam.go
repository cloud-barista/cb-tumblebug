package iam

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	// "github.com/labstack/echo-contrib/session"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// This function is required for frontend web server
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

// JWTAuth initializes and returns the JWT middleware.
func JWTAuthMW() echo.MiddlewareFunc {

	log.Debug().Msg("Start - JWTAuthMW")

	signingMethod := viper.GetString("auth.jwt.signing.method")
	if signingMethod == "" {
		log.Error().Msg("signing method is not set")
		return nil
	}

	config := echojwt.Config{
		SigningMethod:  signingMethod,
		KeyFunc:        GetKey,
		SuccessHandler: retrospectToken,
	}

	log.Debug().Msg("End - JWTAuthMW")

	return echojwt.WithConfig(config)
}

// GetKey is the KeyFunc for the JWT middleware to supply the key for verification.
func GetKey(token *jwt.Token) (interface{}, error) {
	log.Debug().Msg("Start - GetKey")

	base64PubKeyStr := viper.GetString("auth.jwt.publickey")
	if base64PubKeyStr == "" {
		return nil, fmt.Errorf("public key is not set")
	}

	publicKey, _ := parseRsaPublicKey(base64PubKeyStr)

	key, _ := jwk.New(publicKey)

	var pubkey interface{}
	if err := key.Raw(&pubkey); err != nil {
		return nil, fmt.Errorf("unable to get the public key. error: %s", err.Error())
	}

	log.Debug().Msg("end - GetKey")

	return pubkey, nil
}

// parseRsaPublicKey parses a base64 encoded public key into an rsa.PublicKey.
func parseRsaPublicKey(base64Str string) (*rsa.PublicKey, error) {
	buf, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, err
	}
	parsedKey, err := x509.ParsePKIXPublicKey(buf)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsedKey.(*rsa.PublicKey)
	if ok {
		return publicKey, nil
	}
	return nil, fmt.Errorf("unexpected key type %T", publicKey)
}

// The SuccessHandler for the JWT middleware
// It will be called if jwt.Parse succeeds and set the claims in the context.
// (Briefly, it is the process of checking whether a (previously) issued token is still valid or not.)
func retrospectToken(c echo.Context) {
	log.Debug().Msg("start - retrospectToken, which is the SuccessHandler")

	// Get the jwtToken from the context
	jwtToken, ok := c.Get("user").(*jwt.Token) // by default token is stored under `user` key
	if !ok {
		c.String(http.StatusBadRequest, "missing or invalid JWT token")
	}

	// Get the claims from the token
	claims, ok := jwtToken.Claims.(jwt.MapClaims) // by default claims is of type `jwt.MapClaims`
	if !ok {
		c.String(http.StatusUnauthorized, "failed to type cast claims as jwt.MapClaims")
	}

	// Get the realm roles from the claims
	roles := ParseRealmRoles(claims)

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
	exp, ok := claims["exp"].(float64)
	if !ok {
		// If the exp claim is missing or not of the expected type
		log.Debug().Msgf("unable to find or parse expiry time from token")
		c.String(http.StatusNotFound, "unable to find or parse expiry time from token")
	}
	expiryTime := time.Unix(int64(exp), 0)         // Unix time
	expiredTime := expiryTime.Format(time.RFC3339) // RFC3339 time

	// log.Trace().Msgf("token: %+v", token)
	log.Trace().Msgf("token.Raw: %+v", jwtToken.Raw)
	log.Trace().Msgf("claims: %+v", claims)

	// Set user as authenticated
	c.Set("authenticated", true)
	c.Set("token", jwtToken.Raw)
	// Set user name
	c.Set("name", claims["name"])
	c.Set("role", role)
	c.Set("expired-time", expiredTime)
	// Set more values here
	// ...

	log.Debug().Msg("End - retrospectToken, which is the SuccessHandler")
}

func ParseRealmRoles(claims jwt.MapClaims) []string {
	var realmRoles []string = make([]string, 0)

	if claim, ok := claims["realm_access"]; ok {
		if roles, ok := claim.(map[string]interface{})["roles"]; ok {
			for _, role := range roles.([]interface{}) {
				realmRoles = append(realmRoles, role.(string))
			}
		}
	}
	return realmRoles
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
