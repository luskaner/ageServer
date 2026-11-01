package ipc

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/gob"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/launcher-common/ipc"
	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal"
)

func resetIPCState(t *testing.T) {
	t.Helper()
	oldMapped := mappedIps
	oldAdded := addedCert
	oldRunSetUp := runSetUpFn
	oldRunRevert := runRevertFn
	oldBuffer := bufferFn
	oldParse := parseCertFn
	oldSetupServer := setupServerFn
	oldRevertServer := revertServerFn
	t.Cleanup(func() {
		mappedIps = oldMapped
		addedCert = oldAdded
		runSetUpFn = oldRunSetUp
		runRevertFn = oldRunRevert
		bufferFn = oldBuffer
		parseCertFn = oldParse
		setupServerFn = oldSetupServer
		revertServerFn = oldRevertServer
	})
	mappedIps = false
	addedCert = false
	bufferFn = func(_ string, fn func(writer io.Writer)) error {
		fn(nil)
		return nil
	}
}

func makeValidCert(gameId string) *x509.Certificate {
	domains := make([]string, len(common.SelfSignedCertDomains))
	copy(domains, common.SelfSignedCertDomains)
	cert := &x509.Certificate{
		SerialNumber:   big.NewInt(1),
		Subject:        pkix.Name{CommonName: "not_valid"},
		IsCA:           true,
		MaxPathLenZero: true,
		DNSNames:       domains,
		KeyUsage:       x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		NotBefore:      time.Now(),
		NotAfter:       time.Now().Add(time.Hour),
	}
	if !common.SelfSignedCertGame(gameId) {
		cert.DNSNames = nil
		cert.MaxPathLenZero = false
		cert.KeyUsage = x509.KeyUsageCertSign
		cert.ExtKeyUsage = []x509.ExtKeyUsage{}
		// For non-game, set simple CA
		cert.IsCA = true
	}
	return cert
}

// -------------------------------------------------------------------
// handleSetUp tests
// -------------------------------------------------------------------

func TestHandleSetUpDecodeFailure(t *testing.T) {
	resetIPCState(t)
	// Create a decoder that will fail to decode SetupCommand (e.g., encode a string instead)
	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode("bad")
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrDecode {
		t.Fatalf("code=%d want ErrDecode", code)
	}
}

func TestHandleSetUpIPsAlreadyMapped(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, IP: net.ParseIP("127.0.0.2")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrIpsAlreadyMapped {
		t.Fatalf("code=%d want ErrIpsAlreadyMapped", code)
	}
}

func TestHandleSetUpGameNotSupported(t *testing.T) {
	resetIPCState(t)
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: "unknown", IP: net.ParseIP("1.1.1.1")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrGameNotSupported {
		t.Fatalf("code=%d want ErrGameNotSupported", code)
	}
}

func TestHandleSetUpCertAlreadyAdded(t *testing.T) {
	resetIPCState(t)
	addedCert = true
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, Certificate: []byte("cert")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrCertAlreadyAdded {
		t.Fatalf("code=%d want ErrCertAlreadyAdded", code)
	}
}

func TestHandleSetUpCertParseFailure(t *testing.T) {
	resetIPCState(t)
	parseCertFn = func([]byte) (*x509.Certificate, error) { return nil, errors.New("parse fail") }
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, Certificate: []byte("bad")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrCertInvalid {
		t.Fatalf("code=%d want ErrCertInvalid", code)
	}
}

func TestHandleSetUpCertInvalid(t *testing.T) {
	resetIPCState(t)
	// Return a cert that fails check (e.g., not CA)
	parseCertFn = func([]byte) (*x509.Certificate, error) {
		return &x509.Certificate{IsCA: false}, nil
	}
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, Certificate: []byte("cert")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != internal.ErrCertInvalid {
		t.Fatalf("code=%d want ErrCertInvalid", code)
	}
}

func TestHandleSetUpBufferFailure(t *testing.T) {
	resetIPCState(t)
	parseCertFn = func([]byte) (*x509.Certificate, error) { return makeValidCert(game.AoE2), nil }
	bufferFn = func(string, func(writer io.Writer)) error { return errors.New("buffer fail") }
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, Certificate: []byte("cert")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != common.ErrFileLog {
		t.Fatalf("code=%d want ErrFileLog", code)
	}
}

func TestHandleSetUpSuccessWithCert(t *testing.T) {
	resetIPCState(t)
	parseCertFn = func([]byte) (*x509.Certificate, error) { return makeValidCert(game.AoE2), nil }
	runSetUpFn = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, Certificate: []byte("cert")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
	if !addedCert {
		t.Fatal("addedCert should be true after success")
	}
}

func TestHandleSetUpSuccessWithIP(t *testing.T) {
	resetIPCState(t)
	runSetUpFn = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, IP: net.ParseIP("127.0.0.2")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
	if !mappedIps {
		t.Fatal("mappedIps should be true")
	}
}

func TestHandleSetUpResultFailure(t *testing.T) {
	resetIPCState(t)
	runSetUpFn = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: 1, Err: errors.New("fail")}
	}
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, IP: net.ParseIP("1.1.1.1")}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
	if mappedIps {
		t.Fatal("mappedIps should stay false on failure")
	}
}

func TestHandleSetUpForwardsCanUseInternet(t *testing.T) {
	resetIPCState(t)
	parseCertFn = func([]byte) (*x509.Certificate, error) { return nil, nil }
	gotCanUseInternet := false
	runSetUpFn = func(_ string, _ net.IP, _ bool, canUseInternet bool, _ *x509.Certificate, _ string, _ io.Writer, _ func(*exec.Options)) *exec.Result {
		gotCanUseInternet = canUseInternet
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	var buf bytes.Buffer
	cmd := ipc.SetupCommand{GameId: game.AoE2, CanUseInternet: true}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleSetUp("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
	if !gotCanUseInternet {
		t.Fatal("canUseInternet should be forwarded to runSetUpFn")
	}
}

// -------------------------------------------------------------------
// handleRevert tests
// -------------------------------------------------------------------

func TestHandleRevertDecodeFailure(t *testing.T) {
	resetIPCState(t)
	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode("bad")
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != internal.ErrDecode {
		t.Fatalf("code=%d want ErrDecode", code)
	}
}

func TestHandleRevertAlreadyReverted(t *testing.T) {
	resetIPCState(t)
	// Neither mappedIps nor addedCert set, so any revert should be already reverted
	var buf bytes.Buffer
	cmd := ipc.RevertCommand{IPs: true, Certificate: true}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
}

func TestHandleRevertBufferFailure(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	addedCert = true
	bufferFn = func(string, func(writer io.Writer)) error { return errors.New("buffer fail") }
	var buf bytes.Buffer
	cmd := ipc.RevertCommand{IPs: true, Certificate: true}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != common.ErrFileLog {
		t.Fatalf("code=%d want ErrFileLog", code)
	}
}

func TestHandleRevertSuccess(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	addedCert = true
	runRevertFn = func(bool, bool, bool, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	var buf bytes.Buffer
	cmd := ipc.RevertCommand{IPs: true, Certificate: false}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
	if mappedIps {
		t.Fatal("mappedIps should be false after revert")
	}
	if !addedCert {
		t.Fatal("addedCert should remain true when only IPs reverted")
	}
}

func TestHandleRevertSuccessBoth(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	addedCert = true
	runRevertFn = func(bool, bool, bool, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	var buf bytes.Buffer
	cmd := ipc.RevertCommand{IPs: true, Certificate: true}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d", code)
	}
	if mappedIps || addedCert {
		t.Fatal("both should be false after revert")
	}
}

func TestHandleRevertFailureKeepsState(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	runRevertFn = func(bool, bool, bool, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: 5}
	}
	var buf bytes.Buffer
	cmd := ipc.RevertCommand{IPs: true}
	gob.NewEncoder(&buf).Encode(cmd)
	dec := gob.NewDecoder(&buf)
	code := handleRevert("", dec)
	if code != 5 {
		t.Fatalf("code=%d want 5", code)
	}
	if !mappedIps {
		t.Fatal("mappedIps should stay true on failure")
	}
}

// -------------------------------------------------------------------
// handleClient tests
// -------------------------------------------------------------------

func TestHandleClientSetupSuccess(t *testing.T) {
	resetIPCState(t)
	runSetUpFn = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	parseCertFn = func([]byte) (*x509.Certificate, error) { return makeValidCert(game.AoE2), nil }

	client, server := net.Pipe()
	defer client.Close()
	done := make(chan bool, 1)
	go func() {
		handleClient("", server)
		done <- true
	}()
	enc := gob.NewEncoder(client)
	dec := gob.NewDecoder(client)
	// Send Setup action
	if err := enc.Encode(ipc.Setup); err != nil {
		t.Fatal(err)
	}
	var ack int
	if err := dec.Decode(&ack); err != nil {
		t.Fatal(err)
	}
	if ack != common.ErrSuccess {
		t.Fatalf("ack=%d", ack)
	}
	cmd := ipc.SetupCommand{GameId: game.AoE2, IP: net.ParseIP("127.0.0.2")}
	if err := enc.Encode(cmd); err != nil {
		t.Fatal(err)
	}
	var code int
	if err := dec.Decode(&code); err != nil {
		t.Fatal(err)
	}
	if code != common.ErrSuccess {
		t.Fatalf("code=%d", code)
	}
	// Send Exit to close
	if err := enc.Encode(ipc.Exit); err != nil {
		t.Fatal(err)
	}
	// handleClient will encode final exit code before returning, but our client may have closed server side? For Exit, it closes connection and encodes.
	// Instead just close and wait
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleClient didn't exit")
	}
}

func TestHandleClientRevertSuccess(t *testing.T) {
	resetIPCState(t)
	mappedIps = true
	runRevertFn = func(bool, bool, bool, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	client, server := net.Pipe()
	done := make(chan bool, 1)
	go func() { handleClient("", server); done <- true }()
	enc := gob.NewEncoder(client)
	dec := gob.NewDecoder(client)
	if err := enc.Encode(ipc.Revert); err != nil {
		t.Fatal(err)
	}
	var ack int
	dec.Decode(&ack)
	if ack != common.ErrSuccess {
		t.Fatalf("ack=%d", ack)
	}
	cmd := ipc.RevertCommand{IPs: true}
	enc.Encode(cmd)
	var code int
	dec.Decode(&code)
	if code != common.ErrSuccess {
		t.Fatalf("code=%d", code)
	}
	// Send Exit: the server now acknowledges before closing the connection so
	// a non-elevated client can tell "still shutting down" from "never got the
	// command". Drain that ack, then wait for it to return.
	enc.Encode(ipc.Exit)
	var exitAck int
	if err := dec.Decode(&exitAck); err != nil {
		t.Fatal(err)
	}
	if exitAck != common.ErrSuccess {
		t.Fatalf("exitAck=%d", exitAck)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestHandleClientExitSuccess(t *testing.T) {
	resetIPCState(t)
	client, server := net.Pipe()
	done := make(chan bool, 1)
	go func() { handleClient("", server); done <- true }()
	enc := gob.NewEncoder(client)
	if err := enc.Encode(ipc.Exit); err != nil {
		t.Fatal(err)
	}
	// The server acknowledges the shutdown before closing, so the client must
	// read the ack instead of seeing an EOF.
	var ack int
	dec := gob.NewDecoder(client)
	if err := dec.Decode(&ack); err != nil {
		t.Fatal(err)
	}
	if ack != common.ErrSuccess {
		t.Fatalf("ack=%d", ack)
	}
	// Then the connection is closed, so the client sees EOF.
	var trailing int
	if err := dec.Decode(&trailing); err == nil {
		t.Error("expected EOF after the Exit ack")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleClient didn't exit on Exit")
	}
	_ = client.Close()
}

func TestHandleClientUnknownAction(t *testing.T) {
	resetIPCState(t)
	client, server := net.Pipe()
	done := make(chan bool, 1)
	go func() { handleClient("", server); done <- true }()
	enc := gob.NewEncoder(client)
	dec := gob.NewDecoder(client)
	// Send unknown action 99
	enc.Encode(byte(99))
	var ack int
	// It will not encode ack for unknown? Actually code does: for !exit loop, it decodes action, then switch, default does nothing, then encodes exitCode (ErrNonExistingAction)
	// So we should get that
	if err := dec.Decode(&ack); err != nil {
		t.Fatal(err)
	}
	if ack != internal.ErrNonExistingAction {
		t.Fatalf("ack=%d want ErrNonExistingAction", ack)
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestHandleClientEncodeFailureOnAck(t *testing.T) {
	resetIPCState(t)
	// Use a conn that fails on write for encoder
	client, server := net.Pipe()
	// Close client immediately to cause server's Encode to fail
	client.Close()
	done := make(chan bool, 1)
	go func() { handleClient("", server); done <- true }()
	// Server will try to decode action, get EOF and exit
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("should exit on EOF")
	}
}

// -------------------------------------------------------------------
// StartServer tests
// -------------------------------------------------------------------

type mockListener struct {
	acceptCh chan net.Conn
	closeCh  chan bool
}

func (m *mockListener) Accept() (net.Conn, error) {
	conn, ok := <-m.acceptCh
	if !ok {
		return nil, errors.New("closed")
	}
	return conn, nil
}
func (m *mockListener) Close() error   { close(m.closeCh); return nil }
func (m *mockListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestStartServerListenFailure(t *testing.T) {
	resetIPCState(t)
	setupServerFn = func() (net.Listener, error) { return nil, errors.New("listen fail") }
	code := StartServer("")
	if code != internal.ErrListen {
		t.Fatalf("code=%d want ErrListen", code)
	}
}

func TestStartServerAcceptAndExit(t *testing.T) {
	resetIPCState(t)
	ml := &mockListener{acceptCh: make(chan net.Conn, 1), closeCh: make(chan bool, 1)}
	setupServerFn = func() (net.Listener, error) { return ml, nil }
	revertServerFn = func() {}
	// Prepare a client conn that will send Exit
	client, server := net.Pipe()
	ml.acceptCh <- server
	close(ml.acceptCh) // after one accept, next Accept will error but loop will continue? Actually StartServer loops forever until handleClient returns true (Exit)
	go func() {
		time.Sleep(100 * time.Millisecond)
		enc := gob.NewEncoder(client)
		enc.Encode(ipc.Exit)
		var code int
		gob.NewDecoder(client).Decode(&code)
		client.Close()
	}()
	code := StartServer("")
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success", code)
	}
}
