package battleServer

import (
	"bytes"
	"encoding/binary"
	"testing"

	battleServerBroadcast "github.com/luskaner/ageLANServer/battle-server-broadcast"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/uuid"
)

func buildBroadcast(t *testing.T, id uuid.UUID, name string, public, ws, oob uint16, declaredNameLength *uint16) []byte {
	t.Helper()
	idText, err := id.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	nameLength := uint16(len(name))
	if declaredNameLength != nil {
		nameLength = *declaredNameLength
	}
	data := make([]byte, 0, len(battleServerBroadcast.Header)+battleServerBroadcast.GuidLength+2+len(name)+6)
	data = append(data, battleServerBroadcast.Header...)
	data = append(data, idText...)
	var lenBuf [2]byte
	binary.LittleEndian.PutUint16(lenBuf[:], nameLength)
	data = append(data, lenBuf[:]...)
	data = append(data, name...)
	for _, port := range []uint16{public, ws, oob} {
		binary.LittleEndian.PutUint16(lenBuf[:], port)
		data = append(data, lenBuf[:]...)
	}
	return data
}

func TestParseBroadcastMessageValid(t *testing.T) {
	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	data := buildBroadcast(t, id, "My Server", 20001, 20002, 20003, nil)

	msg, err := ParseBroadcastMessage(data, len(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Id != id {
		t.Errorf("Id = %v, want %v", msg.Id, id)
	}
	if msg.Name != "My Server" {
		t.Errorf("Name = %q", msg.Name)
	}
	if msg.PublicPort != 20001 || msg.WebsocketPort != 20002 || msg.OutOfBandPort != 20003 {
		t.Errorf("ports = %d/%d/%d", msg.PublicPort, msg.WebsocketPort, msg.OutOfBandPort)
	}
}

func TestParseBroadcastMessageTooShort(t *testing.T) {
	if _, err := ParseBroadcastMessage(make([]byte, battleServerBroadcast.MinimumSize-1), battleServerBroadcast.MinimumSize-1); err == nil {
		t.Fatal("expected error for too-short data")
	}
}

func TestParseBroadcastMessageInvalidHeader(t *testing.T) {
	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	data := buildBroadcast(t, id, "srv", 1, 2, 3, nil)
	data[0] ^= 0xFF
	if _, err := ParseBroadcastMessage(data, len(data)); err == nil {
		t.Fatal("expected error for invalid header")
	}
}

func TestParseBroadcastMessageUnparsableGuid(t *testing.T) {
	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	data := buildBroadcast(t, id, "srv", 1, 2, 3, nil)
	copy(data[len(battleServerBroadcast.Header):], bytes.Repeat([]byte{'z'}, battleServerBroadcast.GuidLength))
	if _, err := ParseBroadcastMessage(data, len(data)); err == nil {
		t.Fatal("expected error for unparsable GUID")
	}
}

// Regression: a mismatched name length used to keep parsing and panic on slice bounds.
func TestParseBroadcastMessageSizeMismatchNoPanic(t *testing.T) {
	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	huge := uint16(60000)
	data := make([]byte, battleServerBroadcast.MinimumSize)
	copy(data, battleServerBroadcast.Header)
	idText, _ := id.MarshalText()
	copy(data[len(battleServerBroadcast.Header):], idText)
	binary.LittleEndian.PutUint16(data[len(battleServerBroadcast.Header)+battleServerBroadcast.GuidLength:], huge)

	msg, err := ParseBroadcastMessage(data, len(data))
	if err == nil {
		t.Fatal("expected size-mismatch error")
	}
	if msg != nil {
		t.Fatalf("expected nil message on error, got %+v", msg)
	}
}

func TestParseBroadcastMessageDeclaredLengthMismatch(t *testing.T) {
	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	shorter := uint16(2)
	data := buildBroadcast(t, id, "abcd", 1, 2, 3, &shorter)
	if _, err := ParseBroadcastMessage(data, len(data)); err == nil {
		t.Fatal("expected error when declared name length does not match payload")
	}
}

func TestBroadcastPort(t *testing.T) {
	if BroadcastPort(game.AoE1) != 8888 {
		t.Fatal("AoE1 must use port 8888")
	}
	for _, g := range []string{game.AoE2, game.AoE3, game.AoE4, game.AoM, ""} {
		if BroadcastPort(g) != 9999 {
			t.Fatalf("%s must use port 9999", g)
		}
	}
}

func TestNameAndParseFileNameRoundTrip(t *testing.T) {
	for _, index := range []int{0, 1, 42} {
		name := Name(index)
		got, err := ParseFileName(name)
		if err != nil {
			t.Fatalf("ParseFileName(%q): %v", name, err)
		}
		if got != index {
			t.Fatalf("round trip = %d, want %d", got, index)
		}
	}
}

func TestParseFileNameInvalid(t *testing.T) {
	if _, err := ParseFileName("notes.txt"); err == nil {
		t.Fatal("expected error for non-index file name")
	}
}
