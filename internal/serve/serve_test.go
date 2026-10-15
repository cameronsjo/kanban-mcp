package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("passed"))
	})
}

func TestBearerGateRejectsMissingOrWrongToken(t *testing.T) {
	gate := BearerGate("s3cret", okHandler())

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"bare token without scheme", "s3cret", http.StatusUnauthorized},
		{"correct", "Bearer s3cret", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			gate.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestHandlerAuthenticationIncludesHealthz(t *testing.T) {
	for _, tc := range []struct {
		token, header string
		want          int
	}{
		{"", "", http.StatusOK}, {"s3cret", "", http.StatusUnauthorized},
		{"s3cret", "Bearer wrong", http.StatusUnauthorized}, {"s3cret", "Bearer s3cret", http.StatusOK},
	} {
		h := Handler(nil, tc.token)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Authorization", tc.header)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("healthz=%d want=%d", rec.Code, tc.want)
		}
	}
}

func TestHTTPRejectsUnsafeConfigurationBeforeBinding(t *testing.T) {
	for _, tc := range []struct {
		addr, token string
		remote      bool
	}{
		{"0.0.0.0:0", "", false}, {"0.0.0.0:0", "", true},
		{"127.0.0.1:0", " ", true}, {"localhost:0", "token", false},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := HTTP(ctx, tc.addr, tc.token, tc.remote, nil); err == nil {
			t.Fatalf("unsafe listener accepted: %s remote=%v", tc.addr, tc.remote)
		}
	}
}
