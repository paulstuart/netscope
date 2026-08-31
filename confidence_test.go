//go:build linux

package netscope

import "testing"

func TestConfidenceString(t *testing.T) {
	cases := []struct {
		c    Confidence
		want string
	}{
		{High, "High"},
		{Medium, "Medium"},
		{Inferred, "Inferred"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.c.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
