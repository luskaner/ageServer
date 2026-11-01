package playfab

import (
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
