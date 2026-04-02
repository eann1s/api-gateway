package main

import (
	"fmt"
	"github.com/eann1s/rate-limiter/backend/internal/config"
)

func main() {
	cfg, err := config.Load("./config.yml")
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	fmt.Printf("OK routes=%+v\n", cfg.Routes)
}
