//go:build !linux

package netscope

// currentNetNS returns 0 on non-Linux platforms as network namespaces are Linux-specific.
func currentNetNS() (uint64, error) {
	return 0, nil
}
