//go:build linux

package netscope

import "testing"

func TestSourceStatusString(t *testing.T) {
	cases := []struct {
		s    SourceStatus
		want string
	}{
		{Ran, "Ran"},
		{Unavailable, "Unavailable"},
		{Empty, "Empty"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
