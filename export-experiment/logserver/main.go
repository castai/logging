package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/castai/logging/components"
)

const (
	addr            = ":8090"
	receivedLogPath = "received.log"
)

func main() {
	f, err := os.OpenFile(receivedLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Fatalf("open %s: %v", receivedLogPath, err)
	}
	defer f.Close()

	var count atomic.Int64

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("gzip: %v", err), http.StatusBadRequest)
			return
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			http.Error(w, fmt.Sprintf("read: %v", err), http.StatusBadRequest)
			return
		}

		var payload components.IngestLogsRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, fmt.Sprintf("unmarshal: %v", err), http.StatusBadRequest)
			return
		}

		for _, entry := range payload.Entries {
			line := fmt.Sprintf("%s\t%s\t%s\n", time.Now().Format(time.RFC3339Nano), r.URL.Path, entry)
			if _, err := f.WriteString(line); err != nil {
				log.Printf("write received.log: %v", err)
			}
			n := count.Add(1)
			fmt.Printf("[%d] received via %s: %q\n", n, r.URL.Path, entry)
		}
		_ = f.Sync()

		w.WriteHeader(http.StatusOK)
	})

	fmt.Printf("logserver listening on %s, writing to %s\n", addr, receivedLogPath)
	if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec
		log.Fatal(err)
	}
}
