package netscope

import "context"

// Source is one mechanism for discovering reachable networks.
type Source interface {
	Name() string
	Level() Level
	Available(context.Context) Availability
	Discover(context.Context) ([]Finding, error)
}
