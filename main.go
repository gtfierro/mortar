package main

import (
	"log"

	"github.com/gtfierro/mortar2/internal/config"
	"github.com/gtfierro/mortar2/internal/logging"
	"github.com/gtfierro/mortar2/internal/server"
)

func main() {
	srv, err := server.NewFromConfig(logging.NewContextWithLogger(), config.NewFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	defer srv.Shutdown()
	go func() {
		log.Fatal(srv.ServeHTTP())
	}()

	<-srv.Done()
}
