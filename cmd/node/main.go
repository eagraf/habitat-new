package main

import (
	"fmt"

	"go.uber.org/fx"
)

func main() {
	fmt.Println("Hello, World!")

	fx.New().Run()
}
