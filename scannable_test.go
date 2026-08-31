//go:build linux

package netscope

import "testing"

func TestScannable(t *testing.T) {
	cases := []struct {
		prefix string
		want   bool
	}{
		{"10.0.0.0/24", true},
		{"192.168.1.0/24", true},
		{"2001:db8::/32", true},
		{"0.0.0.0/0", false},
		{"::/0", false},
		{"127.0.0.0/8", false},
		{"::1/128", false},
		{"169.254.0.0/16", false},
		{"fe80::/10", false},
		{"224.0.0.0/4", false},
		{"ff00::/8", false},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			p := mustPrefix(t, tc.prefix)
			if got := Scannable(p); got != tc.want {
				t.Errorf("Scannable(%s) = %v, want %v", tc.prefix, got, tc.want)
			}
		})
	}
}
