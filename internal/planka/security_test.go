package planka

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type failingBody struct{ err error }

func (b failingBody) Read([]byte) (int, error) { return 0, b.err }
func (b failingBody) Close() error             { return nil }

// TestDoRejectsPathTraversal locks the do() backstop: a path with a parent-dir
// segment is refused before any request reaches the wire, so a mis-validated ID
// can never traverse to another resource (normalizePath does not path.Clean).
func TestDoRejectsPathTraversal(t *testing.T) {
	m := &mockPlanka{tokens: []string{"t"}}
	c, _ := newTestClient(t, m)

	err := c.Get(context.Background(), "/api/cards/1/../../users", nil)
	if err == nil || !strings.Contains(err.Error(), "'..'") {
		t.Fatalf("expected path-traversal rejection, got %v", err)
	}
	if len(m.requests) != 0 {
		t.Fatalf("traversal path must not hit the wire, got %d requests", len(m.requests))
	}
}

func TestRedactURLForLoggingRemovesUserinfo(t *testing.T) {
	got := RedactURLForLogging("https://agent:secret@example.com/planka/api")
	want := "https://example.com/planka/api"
	if got != want {
		t.Fatalf("RedactURLForLogging() = %q, want %q", got, want)
	}
}

func TestRedactURLForLoggingMalformedURLStillRemovesUserinfo(t *testing.T) {
	got := RedactURLForLogging("http://agent:secret@%zz/path")
	if strings.Contains(got, "secret") || strings.Contains(got, "agent:") {
		t.Fatalf("malformed URL retained userinfo: %q", got)
	}
}

func TestRequestErrorRedactsNestedURLUserinfoAndPreservesCause(t *testing.T) {
	wantErr := errors.New("transport failure")
	c := &Client{
		baseURL: "http://agent:secret@example.com",
		token:   "tok",
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, wantErr
		})},
	}

	err := c.Get(context.Background(), "/api/cards/1", nil)
	if err == nil {
		t.Fatal("Get returned nil, want transport error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "agent:") {
		t.Fatalf("request error leaked URL userinfo: %q", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("errors.Is(%v, wantErr) = false", err)
	}
}

func TestResponseReadErrorRedactsURLAndPreservesCause(t *testing.T) {
	wantErr := errors.New("body read failure")
	c := &Client{
		baseURL: "http://agent:secret@example.com",
		token:   "tok",
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       failingBody{err: wantErr},
				Header:     make(http.Header),
			}, nil
		})},
	}

	err := c.Get(context.Background(), "/api/cards/1", nil)
	if err == nil {
		t.Fatal("Get returned nil, want body read error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "agent:") {
		t.Fatalf("response error leaked URL userinfo: %q", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("errors.Is(%v, wantErr) = false", err)
	}
}

func TestLoginResponseReadErrorRedactsURLAndPreservesCause(t *testing.T) {
	wantErr := errors.New("login body read failure")
	c := &Client{
		baseURL: "http://agent:secret@example.com",
		email:   "agent@example.com",
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       failingBody{err: wantErr},
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, err := c.login(context.Background())
	if err == nil {
		t.Fatal("login returned nil, want body read error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "agent:") {
		t.Fatalf("login response error leaked URL userinfo: %q", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("errors.Is(%v, wantErr) = false", err)
	}
}

func TestResponseDecodeErrorRedactsURL(t *testing.T) {
	c := &Client{
		baseURL: "http://agent:secret@example.com",
		token:   "tok",
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not-json")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	var out itemEnvelope[Card]
	err := c.Get(context.Background(), "/api/cards/1", &out)
	if err == nil {
		t.Fatal("Get returned nil, want decode error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "agent:") {
		t.Fatalf("decode error leaked URL userinfo: %q", err)
	}
}

func TestLoginDebugLogRedactsBaseURL(t *testing.T) {
	oldLogger := logger
	var logs bytes.Buffer
	logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() { logger = oldLogger })

	wantErr := errors.New("login transport failure")
	c := &Client{
		baseURL:  "http://agent:secret@example.com",
		email:    "agent@example.com",
		password: "password",
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, wantErr
		})},
	}

	_, err := c.login(context.Background())
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("login error = %v, want wrapped transport error", err)
	}
	if strings.Contains(logs.String(), "secret") || strings.Contains(logs.String(), "agent:") {
		t.Fatalf("login debug log leaked URL userinfo: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "http://example.com") {
		t.Fatalf("login debug log omitted redacted base URL: %q", logs.String())
	}
}

func TestTermsErrorContainsOperatorOnlyGuidance(t *testing.T) {
	message := ErrTermsNotAccepted.Error()
	if strings.Contains(message, "<password>") || strings.Contains(message, "pendingToken") {
		t.Fatalf("terms guidance exposes unsafe details: %q", message)
	}
	for _, want := range []string{"operator-side setup", "PLANKA_AGENT_PASSWORD", "stdin", "never a command-line argument"} {
		if !strings.Contains(message, want) {
			t.Errorf("terms guidance %q missing %q", message, want)
		}
	}
}
