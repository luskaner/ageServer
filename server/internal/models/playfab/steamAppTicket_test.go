package playfab

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestParseSteamIDHexValid(t *testing.T) {
	ticket := make([]byte, 16)
	binary.LittleEndian.PutUint32(ticket[0:4], 16)                 // ownership length (also initialLen, non-wrapper)
	binary.LittleEndian.PutUint32(ticket[4:8], 0)                  // version
	binary.LittleEndian.PutUint64(ticket[8:16], 76561198000000001) // steamID

	got, err := ParseSteamIDHex(hex.EncodeToString(ticket))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 76561198000000001 {
		t.Fatalf("steamID = %d", got)
	}
}

func TestParseSteamIDHexInvalidHex(t *testing.T) {
	if _, err := ParseSteamIDHex("not-hex"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestParseSteamIDHexTooShort(t *testing.T) {
	if _, err := ParseSteamIDHex(hex.EncodeToString([]byte{1, 2, 3})); err == nil {
		t.Fatal("expected error for short ticket")
	}
}

func TestParseSteamIDHexWrapped(t *testing.T) {
	// Wrapper case: initialLen == 20, followed by gcToken(8) + 8 + tokenGenerated(4)
	// + sessionheader(4) + 8 + sessionExternalIP(4) + 4 + clientConnectionTime(4)
	// + clientConnectionCount(4) + ownership length(4), then ownershipLength +
	// version(4) + steamID(8). Total 72 bytes.
	ticket := make([]byte, 72)
	off := 0
	put32 := func(v uint32) {
		binary.LittleEndian.PutUint32(ticket[off:off+4], v)
		off += 4
	}
	put32(20)                                           // initialLen (wrapper)
	binary.LittleEndian.PutUint64(ticket[off:off+8], 0) // gcToken
	off += 8
	off += 8                                                    // skip 8
	put32(0)                                                    // tokenGenerated
	put32(0)                                                    // sessionheader
	off += 8                                                    // skip 8
	put32(0)                                                    // sessionExternalIP
	off += 4                                                    // skip 4
	put32(0)                                                    // clientConnectionTime
	put32(0)                                                    // clientConnectionCount
	put32(16)                                                   // ownership section length
	put32(16)                                                   // ownershipLength
	put32(0)                                                    // version
	binary.LittleEndian.PutUint64(ticket[off:off+8], 123456789) // steamID

	got, err := ParseSteamIDHex(hex.EncodeToString(ticket))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 123456789 {
		t.Fatalf("steamID = %d", got)
	}
}

func TestIsPlaintextSteamTicketGbeWrapped(t *testing.T) {
	// Same wrapper layout ParseSteamIDHex walks, with a valid SteamID64 and
	// the V4 version gbe writes.
	ticket := make([]byte, 72)
	off := 0
	put32 := func(v uint32) {
		binary.LittleEndian.PutUint32(ticket[off:off+4], v)
		off += 4
	}
	put32(20)                                           // GC length
	binary.LittleEndian.PutUint64(ticket[off:off+8], 0) // gcToken
	off += 8
	off += 8 // padding
	put32(0) // tokenGenerated
	put32(0) // sessionheader
	off += 8 // padding
	put32(0) // sessionExternalIP
	off += 4 // padding
	put32(0) // clientConnectionTime
	put32(0) // clientConnectionCount
	put32(16)
	put32(16) // ownership length
	put32(4)  // version
	binary.LittleEndian.PutUint64(ticket[off:off+8], 76561198000000001)

	if !IsPlaintextSteamTicket(hex.EncodeToString(ticket)) {
		t.Fatal("gbe-style wrapped ticket should be plaintext")
	}
}

func TestIsPlaintextSteamTicketGbeEncryptedAppTicket(t *testing.T) {
	// EncryptedAppTicket de 79 B generado por gbe_fork (sobre v1 y blob en
	// claro): userdata "RLINK", V2 con SteamID 76561197987692763.
	const token = "CAEYBSA3Kkc3AAAAAQAAAAUAAAAAAAAAUkxJTksyAAAABAAAANuAogEBABABAAAAAAAAAAAAAAAAAAAAANbF/GpWdRhrAQAAAAAAAAAAAA=="
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if !IsPlaintextSteamTicket(hex.EncodeToString(raw)) {
		t.Fatal("gbe encrypted app ticket is plaintext and should be detected")
	}
}

func TestIsPlaintextSteamTicketRejectsEncryptedAndMalformed(t *testing.T) {
	encrypted := make([]byte, 160)
	for i := range encrypted {
		encrypted[i] = 0xa5
	}
	if IsPlaintextSteamTicket(hex.EncodeToString(encrypted)) {
		t.Fatal("random/encrypted bytes must not be treated as plaintext")
	}
	if IsPlaintextSteamTicket("not-hex") {
		t.Fatal("invalid hex must not be treated as plaintext")
	}
	if IsPlaintextSteamTicket("") {
		t.Fatal("empty ticket must not be treated as plaintext")
	}
}

func oldGbeToken(steamID uint64) string {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data[0:4], 20)
	binary.LittleEndian.PutUint64(data[12:20], steamID)
	return base64.StdEncoding.EncodeToString(data)
}

func gbeToken(steamID uint64, gc bool, session bool) string {
	lengthOffset := 0
	if gc {
		lengthOffset = 24 + 4
	}
	if session {
		lengthOffset = 24 + 28 + 4
	}
	const layoutLen = 4 + 42
	data := make([]byte, lengthOffset+layoutLen+128)
	binary.LittleEndian.PutUint32(data[lengthOffset:lengthOffset+4], layoutLen)
	blob := lengthOffset + 4
	binary.LittleEndian.PutUint32(data[blob:blob+4], 4)
	binary.LittleEndian.PutUint64(data[blob+4:blob+12], steamID)
	return base64.StdEncoding.EncodeToString(data)
}

func TestSteamIDFromToken(t *testing.T) {
	const steamID uint64 = 76561198000000001
	cases := map[string]string{
		"old layout":              oldGbeToken(steamID),
		"new layout without GC":   gbeToken(steamID, false, false),
		"new layout with GC":      gbeToken(steamID, true, false),
		"new layout with session": gbeToken(steamID, true, true),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := SteamIDFromToken(token)
			if !ok || got != steamID {
				t.Fatalf("SteamIDFromToken(%s) = %d, %v; want %d, true", name, got, ok, steamID)
			}
		})
	}
}

func TestSteamIDFromTokenLeavesOpaqueTicketsAlone(t *testing.T) {
	opaque := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa5}, 160))
	for _, token := range []string{opaque, "not-base64", ""} {
		if got, ok := SteamIDFromToken(token); ok {
			t.Fatalf("SteamIDFromToken(%q) = %d, true; want false", token, got)
		}
	}
}
