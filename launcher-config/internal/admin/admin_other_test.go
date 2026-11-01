//go:build !windows

package admin

import (
	"errors"
	"net"
	"os"
	"testing"
)

func TestIsAccessDeniedOther(t *testing.T) {
	if !isAccessDenied(os.ErrPermission) {
		t.Error("os.ErrPermission should be reported as an access denial")
	}
	if isAccessDenied(nil) {
		t.Error("nil is not an access denial")
	}
	if isAccessDenied(net.ErrClosed) {
		t.Error("net.ErrClosed is not an access denial")
	}
	if isAccessDenied(errors.New("something else")) {
		t.Error("an unrelated error is not an access denial")
	}
}
