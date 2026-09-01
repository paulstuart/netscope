package netscope

import "testing"

func TestNexthopString(t *testing.T) {
	cases := []struct {
		n    Nexthop
		want string
	}{
		{Connected, "Connected"},
		{Gateway, "Gateway"},
		{Tunnel, "Tunnel"},
		{Advertised, "Advertised"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.n.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
