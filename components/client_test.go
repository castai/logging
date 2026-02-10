package components

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_NewAPIClient(t *testing.T) {
	t.Run("should create new APIClient", func(t *testing.T) {
		_, err := NewAPIClient(Config{
			APIBaseURL: "http://localhost:1234",
			APIKey:     "key",
			ClusterID:  "clusterID",
			Component:  "omni-agent",
			Version:    "123",
		})
		require.NoError(t, err)
	})
	t.Run("should create new APIClient with valid CA cert", func(t *testing.T) {
		caCert := `
-----BEGIN CERTIFICATE-----
MIIDATCCAemgAwIBAgIUPUS4krHP49SF+yYMLHe4nCllKmEwDQYJKoZIhvcNAQEL
BQAwDzENMAsGA1UECgwEVGVzdDAgFw0yMzA5MTMwODM5MzhaGA8yMjE1MDUxMDA4
MzkzOFowDzENMAsGA1UECgwEVGVzdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC
AQoCggEBAOVZbDa4/tf3N3VP4Ezvt18d++xrQ+bzjhuE7MWX36NWZ4wUzgmqQXd0
OQWoxYqRGKyI847v29j2BWG17ZmbqarwZHjR98rn9gNtRJgeURlEyAh1pAprhFwb
IBS9vyyCNJtfFFF+lvWvJcU+VKIqWH/9413xDx+OE8tRWNRkS/1CVJg1Nnm3H/IF
lhWAKOYbeKY9q8RtIhb4xNqIc8nmUjDFIjRTarIuf+jDwfFQAPK5pNci+o9KCDgd
Y4lvnGfvPp9XAHnWzTRWNGJQyefZb/SdJjXlic10njfttzKBXi0x8IuV2x98AEPE
2jLXIvC+UBpvMhscdzPfahp5xkYJWx0CAwEAAaNTMFEwHQYDVR0OBBYEFFE48b+V
4E5PWqjpLcUnqWvDDgsuMB8GA1UdIwQYMBaAFFE48b+V4E5PWqjpLcUnqWvDDgsu
MA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAIe82ddHX61WHmyp
zeSiF25aXBqeOUA0ScArTL0fBGi9xZ/8gVU79BvJMyfkaeBKvV06ka6g9OnleWYB
zhBmHBvCL6PsgwLxgzt/dj5ES0K3Ml+7jGmhCKKryzYj/ZvhSMyLlxZqP/nRccBG
y6G3KK4bjzqY4TcEPNs8H4Akc+0SGcPl+AAe65mXPIQhtMkANFLoRuWxMf5JmJke
dYT1GoOjRJpEWCATM+KCXa3UEpRBcXNLeOHZivuqf7n0e1CUD6+0oK4TLxVsTqti
q276VYI/vYmMLRI/iE7Qjn9uGEeR1LWpVngE9jSzSdzByvzw3DwO4sL5B+rv7O1T
9Qgi/No=
-----END CERTIFICATE-----
		`
		_, err := NewAPIClient(Config{
			APIBaseURL: "http://localhost:1234",
			APIKey:     "key",
			ClusterID:  "clusterID",
			Component:  "omni-agent",
			Version:    "123",
			TLSCert:    caCert,
		})
		require.NoError(t, err)
	})
	t.Run("should return err with invalid CA cert", func(t *testing.T) {
		caCert := "invalid-ca-cert"
		_, err := NewAPIClient(Config{
			APIBaseURL: "http://localhost:1234",
			APIKey:     "key",
			ClusterID:  "clusterID",
			Component:  "omni-agent",
			Version:    "123",
			TLSCert:    caCert,
		})
		require.Error(t, err)
	})

	t.Run("should validate required fields", func(t *testing.T) {
		tests := []struct {
			name   string
			config Config
			errMsg string
		}{
			{
				name:   "missing APIBaseURL",
				config: Config{APIKey: "key", ClusterID: "cluster", Component: "comp", Version: "v1"},
				errMsg: "APIBaseURL is required",
			},
			{
				name:   "missing APIKey",
				config: Config{APIBaseURL: "http://test", ClusterID: "cluster", Component: "comp", Version: "v1"},
				errMsg: "APIKey is required",
			},
			{
				name:   "missing ClusterID",
				config: Config{APIBaseURL: "http://test", APIKey: "key", Component: "comp", Version: "v1"},
				errMsg: "ClusterID is required",
			},
			{
				name:   "missing Component",
				config: Config{APIBaseURL: "http://test", APIKey: "key", ClusterID: "cluster", Version: "v1"},
				errMsg: "Component is required",
			},
			{
				name:   "missing Version",
				config: Config{APIBaseURL: "http://test", APIKey: "key", ClusterID: "cluster", Component: "comp"},
				errMsg: "Version is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewAPIClient(tt.config)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			})
		}
	})
}

func TestClient_IngestLogs(t *testing.T) {
	t.Run("happy path - should send valid request with correct headers and body", func(t *testing.T) {
		// Setup test server
		var receivedRequest *http.Request
		var receivedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequest = r
			var err error
			receivedBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create client
		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
		})
		require.NoError(t, err)

		// Prepare test data
		testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		entries := []Entry{
			{
				Level:   "info",
				Message: "test message 1",
				Time:    testTime,
				Fields: map[string]string{
					"key1": "value1",
				},
			},
			{
				Level:   "error",
				Message: "test message 2",
				Time:    testTime.Add(time.Second),
				Fields: map[string]string{
					"key2": "value2",
				},
			},
		}

		// Execute
		err = client.IngestLogs(context.Background(), entries)
		require.NoError(t, err)

		// Verify request method and URL
		require.NotNil(t, receivedRequest, "server did not receive request")
		require.Equal(t, http.MethodPost, receivedRequest.Method, "wrong HTTP method")
		require.Equal(t, "/v1/clusters/cluster-123/components/test-component/logs", receivedRequest.URL.Path, "wrong URL path")

		// Verify headers
		require.Equal(t, "test-api-key", receivedRequest.Header.Get("X-API-Key"), "API key header missing or incorrect")
		require.Equal(t, "application/json", receivedRequest.Header.Get("Content-Type"), "Content-Type header missing or incorrect")
		require.Equal(t, "gzip", receivedRequest.Header.Get("Content-Encoding"), "Content-Encoding header should be gzip")

		// Decompress gzip body
		gzipReader, err := gzip.NewReader(bytes.NewReader(receivedBody))
		require.NoError(t, err, "failed to create gzip reader")
		defer gzipReader.Close()

		decompressedBody, err := io.ReadAll(gzipReader)
		require.NoError(t, err, "failed to decompress body")

		// Verify body
		var receivedPayload IngestLogsRequest
		err = json.Unmarshal(decompressedBody, &receivedPayload)
		require.NoError(t, err, "failed to parse request body")

		require.Equal(t, "v1.0.0", receivedPayload.Version, "wrong version in payload")
		require.Len(t, receivedPayload.Entries, 2, "wrong number of entries")

		// Verify first entry
		require.Equal(t, "info", receivedPayload.Entries[0].Level)
		require.Equal(t, "test message 1", receivedPayload.Entries[0].Message)
		require.Equal(t, testTime, receivedPayload.Entries[0].Time)
		require.Equal(t, "value1", receivedPayload.Entries[0].Fields["key1"])

		// Verify second entry
		require.Equal(t, "error", receivedPayload.Entries[1].Level)
		require.Equal(t, "test message 2", receivedPayload.Entries[1].Message)
		require.Equal(t, testTime.Add(time.Second), receivedPayload.Entries[1].Time)
		require.Equal(t, "value2", receivedPayload.Entries[1].Fields["key2"])
	})

	t.Run("should handle server error responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid request"))
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
		})
		require.NoError(t, err)

		entries := []Entry{
			{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			},
		}

		err = client.IngestLogs(context.Background(), entries)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ingest logs failed")
		require.Contains(t, err.Error(), "400")
	})

	t.Run("should respect context cancellation", func(t *testing.T) {
		// Create a server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
		})
		require.NoError(t, err)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		entries := []Entry{
			{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			},
		}

		err = client.IngestLogs(ctx, entries)
		require.Error(t, err)
	})

	t.Run("should handle empty entries", func(t *testing.T) {
		var receivedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			receivedBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
		})
		require.NoError(t, err)

		err = client.IngestLogs(context.Background(), []Entry{})
		require.NoError(t, err)

		// Decompress gzip body
		gzipReader, err := gzip.NewReader(bytes.NewReader(receivedBody))
		require.NoError(t, err)
		defer gzipReader.Close()

		decompressedBody, err := io.ReadAll(gzipReader)
		require.NoError(t, err)

		var receivedPayload IngestLogsRequest
		err = json.Unmarshal(decompressedBody, &receivedPayload)
		require.NoError(t, err)
		require.Empty(t, receivedPayload.Entries)
	})

	t.Run("should retry on server errors (5xx)", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				// First 2 attempts fail with 500
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("server error"))
			} else {
				// Third attempt succeeds
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
			MaxRetries: 3, // Allow up to 3 retries
		})
		require.NoError(t, err)

		entries := []Entry{{Level: "info", Message: "test", Time: time.Now()}}
		err = client.IngestLogs(context.Background(), entries)
		require.NoError(t, err)
		require.Equal(t, 3, attemptCount, "should have retried 2 times before succeeding on 3rd attempt")
	})

	t.Run("should not retry on client errors (4xx)", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
			MaxRetries: 3,
		})
		require.NoError(t, err)

		entries := []Entry{{Level: "info", Message: "test", Time: time.Now()}}
		err = client.IngestLogs(context.Background(), entries)
		require.Error(t, err)
		require.Equal(t, 1, attemptCount, "should not retry on 4xx errors")
		require.Contains(t, err.Error(), "400")
	})

	t.Run("should respect max retries", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
			MaxRetries: 2,
		})
		require.NoError(t, err)

		entries := []Entry{{Level: "info", Message: "test", Time: time.Now()}}
		err = client.IngestLogs(context.Background(), entries)
		require.Error(t, err)
		require.Equal(t, 3, attemptCount, "should try initial + 2 retries = 3 attempts")
		require.Contains(t, err.Error(), "500")
	})

	t.Run("should work with disabled retries", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
			MaxRetries: -1, // No retries
		})
		require.NoError(t, err)

		entries := []Entry{{Level: "info", Message: "test", Time: time.Now()}}
		err = client.IngestLogs(context.Background(), entries)
		require.Error(t, err)
		require.Equal(t, 1, attemptCount, "should only try once with 0 retries")
	})

	t.Run("should compress payload with gzip", func(t *testing.T) {
		var compressedSize int
		var decompressedSize int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			compressedBody, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			compressedSize = len(compressedBody)

			// Decompress to get original size
			gzipReader, err := gzip.NewReader(bytes.NewReader(compressedBody))
			require.NoError(t, err)
			defer gzipReader.Close()

			decompressedBody, err := io.ReadAll(gzipReader)
			require.NoError(t, err)
			decompressedSize = len(decompressedBody)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := NewAPIClient(Config{
			APIBaseURL: server.URL,
			APIKey:     "test-api-key",
			ClusterID:  "cluster-123",
			Component:  "test-component",
			Version:    "v1.0.0",
		})
		require.NoError(t, err)

		// Create many entries to make compression effective
		entries := make([]Entry, 100)
		for i := 0; i < 100; i++ {
			entries[i] = Entry{
				Level:   "info",
				Message: "This is a test message that should compress well due to repetition",
				Time:    time.Now(),
				Fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
					"field3": "value3",
				},
			}
		}

		err = client.IngestLogs(context.Background(), entries)
		require.NoError(t, err)

		// Verify compression is working (compressed should be significantly smaller)
		require.Greater(t, decompressedSize, compressedSize, "decompressed size should be larger than compressed size")
		compressionRatio := float64(decompressedSize) / float64(compressedSize)
		require.Greater(t, compressionRatio, 2.0, "compression ratio should be at least 2x for repetitive data")
	})
}
