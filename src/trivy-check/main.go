package main

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
)

func main() {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, _ := token.SignedString([]byte("secret"))
	fmt.Println(tokenString)
}
