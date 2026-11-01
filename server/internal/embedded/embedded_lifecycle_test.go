package embedded

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
)

// writeCertificatePair writes the five PEM files that common.CertificatePairs
// requires, all holding the same self-signed certificate and key, so the test
// does not depend on the certificates of an installed server.
func writeCertificatePair(t *testing.T, dir string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "embedded-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	for _, name := range []string{
		common.Cert, common.Key, common.CACert, common.SelfSignedCert, common.SelfSignedKey,
	} {
		data := certPEM
		if name == common.Key || name == common.SelfSignedKey {
			data = keyPEM
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// lobbyConnectBody builds a first-connect LobbyConnect: sessionID 48 bits with
// titleID on top of them, the peer secret, reconnect state 0 and a client
// message version of 5 followed by the profile ID.
func lobbyConnectBody() []byte {
	body := make([]byte, 31)
	body[6], body[7] = 0xFF, 0xFF // titleID 0xFFFF
	for i := 8; i < 16; i++ {
		body[i] = byte(i) // secret
	}
	body[16] = 0              // reconnect state: a first connect
	body[17], body[18] = 5, 0 // client message version
	body[27] = 0x42           // profile ID
	return body
}

// readMessage reads one RLink message and returns [type][body].
func readMessage(t *testing.T, conn net.Conn) []byte {
	t.Helper()

	var length [2]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		t.Fatalf("read length: %v", err)
	}
	size := int(length[0])<<8 | int(length[1])
	if size <= 0 {
		t.Fatalf("read length: got %d", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return payload
}

func dialWait(t *testing.T, addr string, budget time.Duration) net.Conn {
	t.Helper()

	deadline := time.Now().Add(budget)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial %s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestStartOutlivesTheStartupContext is the regression for the embedded server
// being unable to create a lobby. cmd/root.go used to pass a startup context
// and cancel it as soon as Start returned; the battle server takes the context
// it is started with as its lifetime, so the cancellation closed the bsPort
// listener, closed the out-of-band listener and stopped the dispatch loop: the
// instance stayed registered in the battle server list while every client was
// refused or left unanswered.
//
// The test cancels the startup context exactly as the caller did and then
// requires the stream a client needs: a LobbyConnect sent to bsPort has to come
// back echoed, which only happens if the listener and the dispatch loop are
// both alive.
func TestStartOutlivesTheStartupContext(t *testing.T) {
	certs := t.TempDir()
	writeCertificatePair(t, certs)

	inst := &Instance{Opts: Options{
		GameID: "age2",
		Region: fmt.Sprintf("embedded-lifecycle-%d", time.Now().UnixNano()),
		Name:   "Embedded Lifecycle",
		IPv4:   "auto",
	}}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	if err := inst.Start(startupCtx, certs, ReplayOptions{}); err != nil {
		cancelStartup()
		t.Fatalf("Start: %v", err)
	}
	cancelStartup()
	t.Cleanup(inst.Stop)

	// The listener has to still accept, and it has to answer: the LobbyConnect
	// is dispatched by the loop the cancellation used to stop.
	conn := dialWait(t, fmt.Sprintf("127.0.0.1:%d", inst.bsPort), 2*time.Second)
	defer func() { _ = conn.Close() }()

	body := lobbyConnectBody()
	frame := make([]byte, 0, len(body)+3)
	size := len(body) + 1
	frame = append(frame, byte(size>>8), byte(size), 0x15)
	frame = append(frame, body...)

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write LobbyConnect: %v", err)
	}
	echo := readMessage(t, conn)
	if echo[0] != 0x15 {
		t.Fatalf("echo type = 0x%02X, want 0x15", echo[0])
	}
	if !bytes.Equal(echo[1:], body) {
		t.Fatalf("echo body = %x, want the request back (%x)", echo[1:], body)
	}

	// The out-of-band listener is the other one the cancellation used to close.
	if inst.oobPort != 0 {
		oob := dialWait(t, fmt.Sprintf("127.0.0.1:%d", inst.oobPort), 2*time.Second)
		_ = oob.Close()
	}
}
