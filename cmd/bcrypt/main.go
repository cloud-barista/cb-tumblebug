package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func main() {
	verify := flag.Bool("verify", false, "Verify a password against a hash")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)

	if *verify {
		fmt.Print("Enter hash: ")
		hash, _ := reader.ReadString('\n')
		hash = strings.TrimSpace(hash)

		fmt.Print("Enter password to verify: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		if verifyPassword(password, hash) {
			fmt.Println("Password is valid!")
		} else {
			fmt.Println("Password is invalid!")
		}
		return
	}

	fmt.Print("Enter password to hash: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	hash, err := hashPassword(password)
	if err != nil {
		fmt.Printf("Error hashing password: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bcrypt hash: %s\n", hash)
	fmt.Printf(" - For docker-compose.yaml and .env (with $$): %s\n", strings.ReplaceAll(hash, "$", "$$"))
	fmt.Printf(" - For Dockerfile or environment variables (with ' '): '%s'\n", hash)
}
