package logging

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Env var names read by New.
const (
	EnvJSONLog     = "JSON_LOG"
	EnvLogTimeZone = "LOG_TIMEZONE"
)

// envJSONLog returns true when JSON_LOG parses as a truthy bool. Panics on
// invalid values (matches the MustParseLevel convention).
func envJSONLog() bool {
	v := os.Getenv(EnvJSONLog)
	if v == "" {
		return false
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		panic(fmt.Errorf("logging: parsing %s=%q as bool: %w", EnvJSONLog, v, err))
	}

	return b
}

// envTimeZone returns the location named by LOG_TIMEZONE, or nil if unset.
// Panics if the zone cannot be loaded.
func envTimeZone() *time.Location {
	v := os.Getenv(EnvLogTimeZone)
	if v == "" {
		return nil
	}
	loc, err := time.LoadLocation(v)
	if err != nil {
		panic(fmt.Errorf("logging: loading %s=%q: %w", EnvLogTimeZone, v, err))
	}

	return loc
}
