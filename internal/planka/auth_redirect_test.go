package planka

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/config"
)

func TestLoginRejectsCrossOriginRedirectsWithoutSendingCredentials(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var targetRequests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetRequests.Add(1)
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"item":"redirected-token"}`)
			}))
			t.Cleanup(target.Close)

			var originRequests atomic.Int32
			var originBody atomic.Value
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				originRequests.Add(1)
				body, _ := io.ReadAll(r.Body)
				originBody.Store(string(body))
				http.Redirect(w, r, target.URL+"/redirected", status)
			}))
			t.Cleanup(origin.Close)

			c := NewClient(config.Config{
				BaseURL:       origin.URL,
				AgentEmail:    "agent@example.com",
				AgentPassword: "password-canary",
			}, "test")
			_, err := c.login(context.Background())
			if err == nil {
				t.Fatal("login returned nil, want redirect error")
			}
			apiErr, ok := AsAPIError(err)
			if !ok || apiErr.Status != status {
				t.Fatalf("login error = %v, want APIError status %d", err, status)
			}
			if got := originRequests.Load(); got != 1 {
				t.Fatalf("origin requests = %d, want 1", got)
			}
			if got := targetRequests.Load(); got != 0 {
				t.Fatalf("cross-origin target requests = %d, want 0", got)
			}
			if body, _ := originBody.Load().(string); !strings.Contains(body, "password-canary") {
				t.Fatalf("origin body = %q, want password canary", body)
			}
		})
	}
}

func TestLoginRejectsSameOriginRedirect(t *testing.T) {
	var requests atomic.Int32
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("redirected") == "1" {
			redirectedRequests.Add(1)
		}
		if requests.Load() == 1 {
			http.Redirect(w, r, "http://"+r.Host+"/api/access-tokens?redirected=1", http.StatusTemporaryRedirect)
			return
		}
		_, _ = fmt.Fprint(w, `{"item":"should-not-be-used"}`)
	}))
	t.Cleanup(target.Close)

	c := NewClient(config.Config{
		BaseURL:       target.URL,
		AgentEmail:    "agent@example.com",
		AgentPassword: "password-canary",
	}, "test")
	_, err := c.login(context.Background())
	if err == nil {
		t.Fatal("login returned nil, want same-origin redirect error")
	}
	apiErr, ok := AsAPIError(err)
	if !ok || apiErr.Status != http.StatusTemporaryRedirect {
		t.Fatalf("login error = %v, want APIError status %d", err, http.StatusTemporaryRedirect)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("same-origin requests = %d, want 1", got)
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("same-origin redirected requests = %d, want 0", got)
	}
}

func TestLoginRedirectPolicyOverridesCustomCheckRedirect(t *testing.T) {
	var customRedirects atomic.Int32
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRequests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"item":"redirected-token"}`)
	}))
	t.Cleanup(target.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/redirected", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	c := NewClient(config.Config{
		BaseURL:       origin.URL,
		AgentEmail:    "agent@example.com",
		AgentPassword: "password-canary",
	}, "test")
	custom := origin.Client()
	custom.CheckRedirect = func(*http.Request, []*http.Request) error {
		customRedirects.Add(1)
		return nil
	}
	c.SetHTTPClient(custom)

	_, err := c.login(context.Background())
	if err == nil {
		t.Fatal("login returned nil, want redirect error")
	}
	apiErr, ok := AsAPIError(err)
	if !ok || apiErr.Status != http.StatusTemporaryRedirect {
		t.Fatalf("login error = %v, want APIError status %d", err, http.StatusTemporaryRedirect)
	}
	if got := customRedirects.Load(); got != 0 {
		t.Fatalf("custom CheckRedirect calls = %d, want 0", got)
	}
	if got := targetRequests.Load(); got != 0 {
		t.Fatalf("target requests = %d, want 0", got)
	}
}
