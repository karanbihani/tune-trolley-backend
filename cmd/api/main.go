package main

import (
	"fmt"

	"spotify-collab/internal/config"
	"spotify-collab/internal/server"
)

func main() {
	config.InitCloudinary()

	server := server.NewServer()

	err := server.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}
