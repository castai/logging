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
	APIBaseURL string
	APIKey     string
	ClusterID  string
	Component  string
	Version    string
	TLSCert    string
}

var _ APIClient = (*APIClientImpl)(nil)

type APIClientImpl struct {
	httpClient *http.Client
	clusterID  string
	component  string
	version    string
	baseURL    string
	apiKey     string
}

func NewAPIClient(cfg Config) (*APIClientImpl, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	httpClient, err := createHTTPClient(cfg.TLSCert)
	if err != nil {
		return nil, err
	}
	return &APIClientImpl{
		baseURL:    cfg.APIBaseURL,
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
		clusterID:  cfg.ClusterID,
		component:  cfg.Component,
		version:    cfg.Version,
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
		Version: a.version,
		Entries: entries,
	}

	// Encode JSON.
	var jsonBuf bytes.Buffer
	if err := json.NewEncoder(&jsonBuf).Encode(payload); err != nil {
		return err
	}

	// Compress with gzip using pooled writer.
	var compressedBuf bytes.Buffer
	gzipWriter := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(gzipWriter)

	gzipWriter.Reset(&compressedBuf)
	if _, err := gzipWriter.Write(jsonBuf.Bytes()); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/v1/clusters/%s/components/%s/logs", a.baseURL, a.clusterID, a.component)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &compressedBuf)
	if err != nil {
		return err
	}

	req.Header.Set(headerAPIKey, a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respMsg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ingest logs failed: expected status %d, got %d: %v", http.StatusOK, resp.StatusCode, string(respMsg))
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
