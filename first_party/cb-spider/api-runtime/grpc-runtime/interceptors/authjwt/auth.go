// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package authjwt

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func validateToken(ctx context.Context) (bool, error) {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false, status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return false, status.Errorf(codes.Unauthenticated, "Authorization jwt token is not supplied")
	}

	tokenStr := authHeader[0]

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})

	if err != nil {
		return false, status.Errorf(codes.Unauthenticated, "Parsing jwt token is failed")
	}

	if token.Valid {
		logger := logger.NewLogger()
		var tokenInfo string = "{"
		for key, val := range claims {
			if key == "expire" {

				if getTokenRemainingValidity(val) < 0 {
					return false, status.Errorf(codes.Unauthenticated, "token is expired")
				}

				var timestamp interface{} = val
				t := time.Unix(int64(timestamp.(float64)), 0)
				tokenInfo = tokenInfo + fmt.Sprintf(" %s: %d-%02d-%02dT%02d:%02d:%02d, remainder seconds: %d,", key,
					t.Year(), t.Month(), t.Day(),
					t.Hour(), t.Minute(), t.Second(),
					getTokenRemainingValidity(val),
				)

			} else {
				tokenInfo = tokenInfo + fmt.Sprintf(" %s: %v,", key, val)
			}
		}
		tokenInfo = tokenInfo + " }"
		logger.Debug("token parsing result : ", tokenInfo)

		return true, nil
	}

	return false, status.Errorf(codes.Unauthenticated, "Authorization is failed")
}

func getTokenRemainingValidity(timestamp interface{}) int {
	if validity, ok := timestamp.(float64); ok {
		tm := time.Unix(int64(validity), 0)
		remainder := tm.Sub(time.Now())

		return int(remainder.Seconds())
	}
	return -1
}
