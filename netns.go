//go:build linux

package netscope

import (
	"fmt"
	"os"
)

// currentNetNS returns the inode of /proc/self/ns/net, which identifies
// which network namespace this process occupies. Netlink answers for
// whichever namespace the process is in, so this lets a caller tell the
// host's real posture apart from a container's NAT'd bridge.
func currentNetNS() (uint64, error) {
	link, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return 0, fmt.Errorf("read /proc/self/ns/net: %w", err)
	}
	var inode uint64
	if _, err := fmt.Sscanf(link, "net:[%d]", &inode); err != nil {
		return 0, fmt.Errorf("unexpected netns link format %q: %w", link, err)
	}
	return inode, nil
}
