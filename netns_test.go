package netscope

import (
	"runtime"
	"testing"
)

func TestCurrentNetNS(t *testing.T) {
	inode, err := currentNetNS()
	if runtime.GOOS == "linux" {
		if err != nil {
			t.Fatalf("currentNetNS() error: %v", err)
		}
		if inode == 0 {
			t.Error("currentNetNS() = 0, want a non-zero inode on Linux")
		}
	} else {
		if err != nil {
			t.Fatalf("currentNetNS() error: %v", err)
		}
		if inode != 0 {
			t.Errorf("currentNetNS() = %d, want 0 on non-Linux", inode)
		}
	}
}
