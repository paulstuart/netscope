//go:build linux

package netscope

import "testing"

func TestLevelHas(t *testing.T) {
	cases := []struct {
		name     string
		selected Level
		check    Level
		want     bool
	}{
		{"local in local|listen", Local | Listen, Local, true},
		{"listen in local|listen", Local | Listen, Listen, true},
		{"ask not in local|listen", Local | Listen, Ask, false},
		{"ask in ask", Ask, Ask, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.selected.Has(tc.check); got != tc.want {
				t.Errorf("Has() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	cases := []struct {
		level Level
		want  string
	}{
		{Local, "Local"},
		{Listen, "Listen"},
		{Ask, "Ask"},
		{Local | Listen, "Local|Listen"},
		{Local | Listen | Ask, "Local|Listen|Ask"},
		{0, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.level.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
