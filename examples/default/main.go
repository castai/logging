package main

import (
	"errors"

	"github.com/castai/logging"
)

func main() {
	log := logging.New()

	log.Info("service starting")
	log.WithField("component", "api").Infof("listening on %s", ":8080")
	log.Warn("cache miss")
	log.Errorf("request failed: %v", errors.New("timeout"))
}
