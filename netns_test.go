//go:build linux

package netscope

import "testing"

func TestCurrentNetNS(t *testing.T) {
	inode, err := currentNetNS()
	if err != nil {
		t.Fatalf("currentNetNS() error: %v", err)
	}
	if inode == 0 {
		t.Error("currentNetNS() = 0, want a non-zero inode")
	}
}
