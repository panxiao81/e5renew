package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeRoundTripper struct {
	resp *http.Response
	err  error
}

func (f fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return f.resp, f.err
}

type captureAPILogger struct {
	mu      sync.Mutex
	entries []APILogEntry
	ch      chan struct{}
}

func (c *captureAPILogger) LogAPICall(_ context.Context, entry APILogEntry) error {
	c.mu.Lock()
	c.entries = append(c.entries, entry)
	c.mu.Unlock()
	select {
	case c.ch <- struct{}{}:
	default:
	}
	return nil
}

func (c *captureAPILogger) last() APILogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entries[len(c.entries)-1]
}

func TestAPILoggerTransportRoundTrip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("non graph host bypasses logging", func(t *testing.T) {
		capture := &captureAPILogger{ch: make(chan struct{}, 1)}
		transport := NewAPILoggerTransport(fakeRoundTripper{resp: &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), ContentLength: 0}}, APILoggerConfig{
			APILogService: capture,
			Logger:        logger,
			JobType:       "test",
		})

		req, _ := http.NewRequest(http.MethodGet, "https://example.com/health", nil)
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		require.Equal(t, 204, resp.StatusCode)

		select {
		case <-capture.ch:
			t.Fatal("unexpected log entry for non graph host")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("graph success logs and preserves body", func(t *testing.T) {
		capture := &captureAPILogger{ch: make(chan struct{}, 1)}
		transport := NewAPILoggerTransport(fakeRoundTripper{resp: &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader("abc")), ContentLength: -1}}, APILoggerConfig{
			APILogService: capture,
			Logger:        logger,
			JobType:       "mail",
		})

		reqBody := io.NopCloser(bytes.NewReader([]byte("req")))
		req, _ := http.NewRequest(http.MethodPost, "https://graph.microsoft.com/v1.0/me/messages", reqBody)
		req.ContentLength = 3
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)

		select {
		case <-capture.ch:
		case <-time.After(time.Second):
			t.Fatal("expected async log entry")
		}

		entry := capture.last()
		require.Equal(t, "me/messages", entry.APIEndpoint)
		require.Equal(t, 200, entry.HTTPStatusCode)
		require.True(t, entry.Success)
		require.Equal(t, 3, entry.RequestSize)
		require.Equal(t, 3, entry.ResponseSize)

		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		require.Equal(t, "abc", string(body))
	})

	t.Run("graph transport error logs failure", func(t *testing.T) {
		capture := &captureAPILogger{ch: make(chan struct{}, 1)}
		transport := NewAPILoggerTransport(fakeRoundTripper{err: errors.New("dial failed")}, APILoggerConfig{
			APILogService: capture,
			Logger:        logger,
			JobType:       "mail",
		})

		req, _ := http.NewRequest(http.MethodGet, "https://graph.microsoft.com/v1.0/users", nil)
		_, err := transport.RoundTrip(req)
		require.Error(t, err)

		select {
		case <-capture.ch:
		case <-time.After(time.Second):
			t.Fatal("expected async log entry")
		}

		entry := capture.last()
		require.Equal(t, 0, entry.HTTPStatusCode)
		require.False(t, entry.Success)
		require.NotNil(t, entry.ErrorMessage)
		require.Contains(t, *entry.ErrorMessage, "dial failed")
	})
}

func TestExtractEndpoint(t *testing.T) {
	transport := NewAPILoggerTransport(nil, APILoggerConfig{})
	require.Equal(t, "root", transport.extractEndpoint("/v1.0/"))
	require.Equal(t, "me/messages", transport.extractEndpoint("/v1.0/me/messages/123"))
	require.Equal(t, "users/messages", transport.extractEndpoint("/v1.0/users/abc/messages"))
	require.Equal(t, "groups/members", transport.extractEndpoint("/v1.0/groups/abc/members"))
	require.Equal(t, "sites", transport.extractEndpoint("/v1.0/sites/root"))
}

func TestNewAPILoggerClient(t *testing.T) {
	c := NewAPILoggerClient(APILoggerConfig{})
	require.NotNil(t, c)
	require.NotNil(t, c.Transport)
	require.Equal(t, 30*time.Second, c.Timeout)
}
