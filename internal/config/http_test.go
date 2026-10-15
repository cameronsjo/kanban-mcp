package config

import "testing"

func TestHTTPExposurePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, addr, token string
		remote, ok        bool
	}{
		{"default IPv4", DefaultAddr, "", false, true},
		{"IPv6 loopback", "[::1]:8900", "", false, true},
		{"other loopback", "127.0.0.2:0", "", false, true},
		{"wildcard", "0.0.0.0:8900", "token", false, false},
		{"empty host", ":8900", "token", false, false},
		{"IPv6 wildcard", "[::]:8900", "token", false, false},
		{"LAN", "192.0.2.1:8900", "token", false, false},
		{"DNS loopback", "localhost:8900", "token", false, false},
		{"remote no token", "0.0.0.0:8900", "", true, false},
		{"remote blank token", "0.0.0.0:8900", " \t\n", true, false},
		{"explicit local no token", DefaultAddr, "", true, false},
		{"remote authenticated", "0.0.0.0:8900", "token", true, true},
		{"remote IPv6 authenticated", "[::]:8900", "token", true, true},
		{"bad address", "127.0.0.1", "token", true, false},
		{"bad port", "127.0.0.1:http", "token", true, false},
		{"negative port", "127.0.0.1:-1", "token", true, false},
		{"oversized port", "127.0.0.1:65536", "token", true, false},
		{"host whitespace", " 127.0.0.1:8900", "token", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Config{Addr: tc.addr, AuthToken: tc.token, AllowRemote: tc.remote}).ValidateHTTP()
			if (err == nil) != tc.ok {
				t.Fatalf("accepted=%v want=%v, err=%v", err == nil, tc.ok, err)
			}
		})
	}
}
