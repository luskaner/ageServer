package battleServer

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestValidate_IgnorePid(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 0,
	}
	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp4"}
	}
	if c.ValidateWith(true, nil, dial) {
		t.Error("Validate(ignorePid=true) should pass with valid base fields")
	}
}

func TestValidate_MissingRegion(t *testing.T) {
	c := Config{
		Base: Base{
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 1,
	}
	if c.Validate(true) {
		t.Error("Validate should fail with empty region")
	}
}

func TestValidate_MissingIPv4(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 1,
	}
	if c.Validate(true) {
		t.Error("Validate should fail with empty IPv4")
	}
}

func TestValidate_ZeroBsPort(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        0,
			WebSocketPort: 2,
		},
		PID: 1,
	}
	if c.Validate(true) {
		t.Error("Validate should fail with zero BsPort")
	}
}

func TestValidate_ZeroWebSocketPort(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 0,
		},
		PID: 1,
	}
	if c.Validate(true) {
		t.Error("Validate should fail with zero WebSocketPort")
	}
}

func TestValidate_PidZeroWithoutIgnore(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 0,
	}
	if c.Validate(false) {
		t.Error("Validate(ignorePid=false) should fail with PID=0")
	}
}

func TestValidate_ProcessNotFound(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 9999999,
	}
	if c.Validate(false) {
		t.Error("Validate should fail when process not found")
	}
}

func TestValidate_InjectableProcessFinder(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 42,
	}
	called := false
	finder := func(pid int) (*os.Process, error) {
		called = true
		return &os.Process{Pid: pid}, nil
	}
	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp4"}
	}
	result := c.ValidateWith(false, finder, dial)
	if !called {
		t.Error("custom ProcessFinder was not called")
	}
	if result {
		t.Error("ValidateWith should fail when dial fails")
	}
}

func TestValidate_InjectableDialSuccess(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          addr.IP.String(),
			BsPort:        addr.Port,
			WebSocketPort: addr.Port,
		},
		PID: 42,
	}
	finder := func(pid int) (*os.Process, error) {
		return &os.Process{Pid: pid}, nil
	}
	if !c.ValidateWith(false, finder, nil) {
		t.Error("ValidateWith should succeed with listening ports")
	}
}

func TestValidate_AutoIPv4(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "auto",
			BsPort:        1,
			WebSocketPort: 2,
		},
		PID: 1,
	}
	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp4"}
	}
	if c.ValidateWith(true, nil, dial) {
		t.Error("ValidateWith should fail when dial fails even with auto IPv4")
	}
}

func TestValidate_WithOutOfBandPort(t *testing.T) {
	c := Config{
		Base: Base{
			Region:        "us",
			IPv4:          "127.0.0.1",
			BsPort:        1,
			WebSocketPort: 2,
			OutOfBandPort: 3,
		},
		PID: 1,
	}
	callCount := 0
	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		callCount++
		return nil, &net.OpError{Op: "dial", Net: "tcp4"}
	}
	c.ValidateWith(true, nil, dial)
	if callCount != 1 {
		t.Errorf("expected 1 dial attempt (stops at first failure), got %d", callCount)
	}
}

func TestParseFileName(t *testing.T) {
	index, err := ParseFileName("42.toml")
	if err != nil {
		t.Fatal(err)
	}
	if index != 42 {
		t.Errorf("ParseFileName = %d, want 42", index)
	}
}

func TestName(t *testing.T) {
	if got := Name(7); got != "7.toml" {
		t.Errorf("Name(7) = %q, want %q", got, "7.toml")
	}
}
