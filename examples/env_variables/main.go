package main

import (
	"os"

	"github.com/castai/logging"
)

func main() {
	if err := os.Setenv("JSON_LOG", "true"); err != nil {
		panic(err)
	}

	if err := os.Setenv("LOG_TIMEZONE", "America/Lima"); err != nil {
		panic(err)
	}

	log := logging.New()

	log.Info("service starting")
	log.WithField("component", "api").
		With("port", 8080).
		Info("listening")
	log.Warnf("slow query: %s", "SELECT * FROM huge_table")
}
