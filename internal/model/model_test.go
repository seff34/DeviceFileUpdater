package model

import "testing"

func TestDeviceFailed(t *testing.T) {
	ok := DeviceResult{Files: []FileResult{{Status: Unchanged}, {Status: Created}}}
	if ok.Failed() {
		t.Fatal("expected not failed")
	}
	byFile := DeviceResult{Files: []FileResult{{Status: Failed}}}
	if !byFile.Failed() {
		t.Fatal("expected failed by file")
	}
	byErr := DeviceResult{Error: "connect: refused"}
	if !byErr.Failed() {
		t.Fatal("expected failed by device error")
	}
	byPost := DeviceResult{Post: &PostResult{ExitCode: 1}}
	if !byPost.Failed() {
		t.Fatal("expected failed by post command")
	}
}
