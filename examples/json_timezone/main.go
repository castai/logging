package main

import (
	"os"
	"time"

	"github.com/castai/logging"
)

func main() {
	log := logging.New(
		logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:     logging.MustParseLevel("info"),
			Output:    os.Stdout,
			AddSource: true,
		}),

		// Force UTC regardless of the process timezone. Swap for any
		// *time.Location, e.g. time.LoadLocation("Europe/Vilnius").
		// You can also set your preferred timezone via `LOG_TIMEZONE = Europe/Vilnius` env variable.
		logging.NewTimeZoneHandler(time.UTC),
	)

	log.Info("service starting")
	log.WithField("component", "api").
		With("port", 8080).
		Info("listening")
	log.Warnf("slow query: %s", "SELECT * FROM huge_table")
}
