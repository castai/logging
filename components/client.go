package components

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type LogLevel string

const (
	LogLevelDebug   LogLevel = "LOG_LEVEL_DEBUG"
	LogLevelInfo    LogLevel = "LOG_LEVEL_INFO"
	LogLevelWarning LogLevel = "LOG_LEVEL_WARNING"
	LogLevelError   LogLevel = "LOG_LEVEL_ERROR"
	LogLevelUnknown LogLevel = "LOG_LEVEL_UNKNOWN"
)

type IngestLogsRequest struct {
	Version string  `json:"version"`
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Time    time.Time         `json:"time"`
	Fields  map[string]string `json:"fields"`
}

const (
	headerAPIKey = "X-API-Key" // #nosec G101
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

type APIClient interface {
	IngestLogs(ctx context.Context, entries []Entry) error
}

type Config struct {
	APIBaseURL          string
	APIKey              string
	ClusterID           string
	Component           string
	Version             string
	TLSCert             string
	MaxRetries          int // Number of retries on failure (-1 = no retries)
	MaxRetryBackoffWait time.Duration
}

var _ APIClient = (*APIClientImpl)(nil)

type APIClientImpl struct {
	httpClient *http.Client
	cfg        Config
}

func NewAPIClient(cfg Config) (*APIClientImpl, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MaxRetryBackoffWait == 0 {
		cfg.MaxRetryBackoffWait = 5 * time.Second
	}

	httpClient, err := createHTTPClient(cfg.TLSCert)
	if err != nil {
		return nil, err
	}
	return &APIClientImpl{
		cfg:        cfg,
		httpClient: httpClient,
	}, nil
}

func validateConfig(cfg Config) error {
	if cfg.APIBaseURL == "" {
		return errors.New("field APIBaseURL is required")
	}
	if cfg.APIKey == "" {
		return errors.New("field APIKey is required")
	}
	if cfg.ClusterID == "" {
		return errors.New("field ClusterID is required")
	}
	if cfg.Component == "" {
		return errors.New("field Component is required")
	}
	if cfg.Version == "" {
		return errors.New("field Version is required")
	}
	return nil
}

func (a *APIClientImpl) IngestLogs(ctx context.Context, entries []Entry) error {
	payload := &IngestLogsRequest{
		Version: a.cfg.Version,
		Entries: entries,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling ingest logs request: %w", err)
	}

	maxRetries := a.cfg.MaxRetries
	backoff := 100 * time.Millisecond
	var lastErr error
	if maxRetries < 0 {
		maxRetries = 0
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := backoff * time.Duration(1<<attempt-1)
			if waitTime > a.cfg.MaxRetryBackoffWait {
				waitTime = a.cfg.MaxRetryBackoffWait
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		err := a.doIngestRequest(ctx, jsonBytes)
		if err == nil {
			return nil
		}
		lastErr = err

		if !a.shouldRetry(err, attempt, maxRetries) {
			return err
		}
	}
	return fmt.Errorf("ingest logs failed after %d retries: %w", maxRetries, lastErr)
}

func (a *APIClientImpl) shouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr.statusCode >= 500
	}
	return true
}

type httpError struct {
	statusCode int
	message    string
}

func (e *httpError) Error() string {
	return e.message
}

func (a *APIClientImpl) doIngestRequest(ctx context.Context, jsonBytes []byte) error {
	var compressedBuf bytes.Buffer
	gzipWriter := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(gzipWriter)

	gzipWriter.Reset(&compressedBuf)
	if _, err := gzipWriter.Write(jsonBytes); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/v1/clusters/%s/components/%s/logs", a.cfg.APIBaseURL, a.cfg.ClusterID, a.cfg.Component)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &compressedBuf)
	if err != nil {
		return err
	}

	req.Header.Set(headerAPIKey, a.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respMsg, _ := io.ReadAll(resp.Body)
		return &httpError{
			statusCode: resp.StatusCode,
			message:    fmt.Sprintf("ingest logs failed: expected status %d, got %d: %v", http.StatusOK, resp.StatusCode, string(respMsg)),
		}
	}
	return nil
}

func createHTTPClient(tlsCert string) (*http.Client, error) {
	tlsConfig, err := createTLSConfig(tlsCert)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		},
	}, nil
}

func createTLSConfig(tlsCert string) (*tls.Config, error) {
	if tlsCert == "" {
		return nil, nil
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM([]byte(tlsCert)) {
		return nil, fmt.Errorf("failed to add root certificate to CA pool")
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}, nil
}
