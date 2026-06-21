package planka

import "testing"

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		secs float64
		want string
	}{
		{0, "0s"},
		{45, "45s"},
		{60, "1m 0s"},
		{90, "1m 30s"},
		{3600, "1h 0m 0s"},
		{3661, "1h 1m 1s"},
		{7384, "2h 3m 4s"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.secs); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}
