package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var (
	jwtKey   = flag.String("jwt-key", "", "The JWT Signing Key")
	userName = flag.String("user", "HongGilDong", "The User Name")
	orgName  = flag.String("org", "ETRI", "The Organization Name")
	clientIP = flag.String("client-ip", "127.0.0.1", "The Client IP Address")
	expire   = flag.Int("expire", 3650, "The Expire Days")
)

func main() {
	flag.Parse()

	if *jwtKey == "" {
		log.Fatalf("jwt key required..")
	}

	tokenStr, err := generateJWT()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("token : %s\n", tokenStr)

	fmt.Printf("----------------------------\n")
	extractJWT(tokenStr)
}

func generateJWT() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)

	claims["userName"] = *userName
	claims["orgName"] = *orgName
	claims["clientIP"] = *clientIP
	claims["expire"] = time.Now().AddDate(0, 0, *expire).Unix()
	tokenString, err := token.SignedString([]byte(*jwtKey))

	if err != nil {
		return "", fmt.Errorf("JWT Generation Error : %s", err.Error())
	}

	return tokenString, nil
}

func extractJWT(tokenStr string) {

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(*jwtKey), nil
	})

	if err != nil {
		log.Fatal(err)
	}

	if token.Valid {
		for key, val := range claims {
			if key == "expire" {
				var timestamp interface{} = val
				t := time.Unix(int64(timestamp.(float64)), 0)
				fmt.Printf("Key: %v, %d-%02d-%02dT%02d:%02d:%02d, remainder seconds: %d\n", key,
					t.Year(), t.Month(), t.Day(),
					t.Hour(), t.Minute(), t.Second(),
					getTokenRemainingValidity(val),
				)

			} else {
				fmt.Printf("Key: %v, value: %v\n", key, val)
			}
		}
	}

}

func getTokenRemainingValidity(timestamp interface{}) int {
	if validity, ok := timestamp.(float64); ok {
		tm := time.Unix(int64(validity), 0)
		remainder := tm.Sub(time.Now())

		return int(remainder.Seconds())
	}
	return -1
}
