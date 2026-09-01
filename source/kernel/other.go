//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

package kernel

import (
	"errors"
)

type otherAdapter struct{}

func newNetlinkAdapter() netlinkClient { return otherAdapter{} }

func (otherAdapter) Links() ([]rawLink, error)       { return nil, errors.New("unsupported platform") }
func (otherAdapter) Addrs() ([]rawAddr, error)       { return nil, errors.New("unsupported platform") }
func (otherAdapter) Routes() ([]rawRoute, error)     { return nil, errors.New("unsupported platform") }
func (otherAdapter) Neighbours() ([]rawNeigh, error) { return nil, errors.New("unsupported platform") }
