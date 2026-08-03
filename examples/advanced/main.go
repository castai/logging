package main

import (
	"os"
	"time"

	"github.com/castai/logging"
	"golang.org/x/time/rate"
)

func main() {
	log := logging.New(
		logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:     logging.MustParseLevel("info"),
			Output:    os.Stdout,
			AddSource: false,
		}),
		logging.NewRateLimitHandler(logging.RateLimiterHandlerConfig{Limit: rate.Limit(2), Burst: 3}),

		// Force UTC regardless of the process timezone. Swap for any
		// *time.Location, e.g. time.LoadLocation("Europe/Vilnius").
		// You can also set your preferred timezone via `LOG_TIMEZONE = Europe/Vilnius` env variable.
		logging.NewTimeZoneHandler(time.UTC),

		// Attaches "commit" to every record, resolved once here rather than
		// per log call.
		logging.NewCommitHandler(),
	)

	log.Info("service starting")

	log = log.WithGroup("api")

	log.With("port", 8080).Info("listening")

	for range 100 {
		log.WithField("arg", "foo").Warnf("slow query: %s", "SELECT * FROM huge_table")
	}
}
