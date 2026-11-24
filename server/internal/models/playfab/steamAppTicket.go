package playfab

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
)

func readUint32(b []byte, off *int) (uint32, bool) {
	if *off+4 > len(b) {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(b[*off : *off+4])
	*off += 4
	return v, true
}

func readUint64(b []byte, off *int) (uint64, bool) {
	if *off+8 > len(b) {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(b[*off : *off+8])
	*off += 8
	return v, true
}

func readVarint(b []byte) (uint64, int, error) {
	var value uint64
	for i := 0; i < len(b) && i < 10; i++ {
		c := b[i]
		value |= uint64(c&0x7f) << (7 * uint(i))
		if c&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, errors.New("truncated varint")
}

// parseSteamID decodes the plaintext layouts understood by ParseSteamIDHex and
// reports whether the 20-byte GC wrapper was present.
func parseSteamID(data []byte) (uint64, bool, error) {
	off := 0

	initialLen, ok := readUint32(data, &off)
	if !ok {
		return 0, false, errors.New("ticket too short")
	}

	wrapped := initialLen == 20
	if wrapped {
		// wrapper case: read wrapper fields like node parser
		if _, ok := readUint64(data, &off); !ok {
			return 0, false, errors.New("unexpected end (gcToken)")
		}
		// skip 8
		off += 8
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (tokenGenerated)")
		}
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (sessionheader)")
		}
		off += 8
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (sessionExternalIP)")
		}
		off += 4
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (clientConnectionTime)")
		}
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (clientConnectionCount)")
		}
		if _, ok := readUint32(data, &off); !ok {
			return 0, false, errors.New("unexpected end (ownership section length)")
		}
	} else {
		// rewind the 4 bytes we read
		off -= 4
	}

	// ownership ticket length
	if _, ok := readUint32(data, &off); !ok {
		return 0, false, errors.New("unexpected end (ownershipLength)")
	}

	// version
	if _, ok := readUint32(data, &off); !ok {
		return 0, false, errors.New("unexpected end (version)")
	}

	// steamID
	steamID, ok := readUint64(data, &off)
	if !ok {
		return 0, false, errors.New("unexpected end (steamID)")
	}

	return steamID, wrapped, nil
}

// ParseSteamIDHex decodes a ticket provided as a hex string and returns the SteamID (uint64).
func ParseSteamIDHex(ticketHex string) (uint64, error) {
	data, err := hex.DecodeString(ticketHex)
	if err != nil {
		return 0, err
	}
	steamID, _, err := parseSteamID(data)
	return steamID, err
}

// IsPlaintextSteamTicket reports whether ticketHex decodes as a plaintext
// ticket, such as the ones emulators like gbe_fork produce. Encrypted Valve
// tickets, random bytes and malformed hex return false.
//
// It is deliberately stricter than ParseSteamIDHex: the parsed SteamID must
// look like an individual account and the surrounding layout must be
// consistent, so an encrypted ticket does not pass by accident.
func IsPlaintextSteamTicket(ticketHex string) bool {
	data, err := hex.DecodeString(ticketHex)
	if err != nil {
		return false
	}
	return isPlaintextOwnershipTicket(data) || isPlaintextEncryptedAppTicket(data)
}

func isPlaintextOwnershipTicket(data []byte) bool {
	steamID, wrapped, err := parseSteamID(data)
	if err != nil || !looksLikeIndividualSteamID(steamID) {
		return false
	}
	if wrapped {
		// GC (20) + session (24) + ownership section: 72 B is the smallest
		// wrapper layout ParseSteamIDHex can walk.
		return len(data) >= 72
	}
	if len(data) < 16 {
		return false
	}
	ownershipLength := binary.LittleEndian.Uint32(data[0:4])
	version := binary.LittleEndian.Uint32(data[4:8])
	if version < 1 || version > 4 {
		return false
	}
	if ownershipLength < 12 {
		return false
	}
	return uint64(ownershipLength)+4 <= uint64(len(data))
}

func isPlaintextEncryptedAppTicket(data []byte) bool {
	var version uint64
	var blob []byte

	for off := 0; off < len(data); {
		key, n, err := readVarint(data[off:])
		if err != nil {
			return false
		}
		off += n
		field, wire := key>>3, key&7
		switch wire {
		case 0:
			value, n, err := readVarint(data[off:])
			if err != nil {
				return false
			}
			off += n
			if field == 1 {
				version = value
			}
		case 2:
			length, n, err := readVarint(data[off:])
			if err != nil {
				return false
			}
			off += n
			if length > uint64(len(data)-off) {
				return false
			}
			if field == 5 {
				blob = data[off : off+int(length)]
			}
			off += int(length)
		default:
			return false
		}
	}

	if version != 1 && version != 2 {
		return false
	}
	if len(blob) < 56 {
		return false
	}
	// V1 header: TicketSize (which is the rest of the blob), version 1 and
	// the userdata length.
	if int(binary.LittleEndian.Uint32(blob[0:4])) != len(blob)-16 {
		return false
	}
	if binary.LittleEndian.Uint32(blob[4:8]) != 1 {
		return false
	}
	userDataLen := binary.LittleEndian.Uint32(blob[8:12])
	if uint64(userDataLen) > uint64(len(blob)-16-40) {
		return false
	}
	v2 := blob[16+int(userDataLen):]
	if len(v2) < 40 {
		return false
	}
	v2Version := binary.LittleEndian.Uint32(v2[4:8])
	if v2Version != 3 && v2Version != 4 {
		return false
	}
	return looksLikeIndividualSteamID(binary.LittleEndian.Uint64(v2[8:16]))
}

// looksLikeIndividualSteamID checks the public universe and individual account
// type bits, plus a non-zero account id. Encrypted data satisfies this with
// probability around 1/4096, which the structural checks above reduce further.
func looksLikeIndividualSteamID(id uint64) bool {
	return id>>56 == 1 && (id>>52)&0xF == 1 && id&0xFFFFFFFF != 0
}

// Steam auth tickets, the tokens the game sends as `Authorization: Steam1.0`.
// gbe_fork has two plaintext layouts; a real Steam ticket is opaque.
const (
	gbeTokenOldSize         = 24
	gbeTokenOldHeader       = 20
	gbeTokenOldSteamIDAt    = 12
	gbeTokenGCSize          = 24
	gbeTokenSessionSize     = 28
	gbeTokenVersion         = 4
	gbeTokenStaticLen       = 42
	gbeTokenSignatureLength = 128
)

// steamTokenEncodings are the base64 alphabets a client may use for the token.
var steamTokenEncodings = []*base64.Encoding{
	base64.StdEncoding,
	base64.RawStdEncoding,
	base64.URLEncoding,
	base64.RawURLEncoding,
}

// SteamIDFromToken extracts the SteamID64 of a base64 `Steam1.0` token when the
// token uses one of the plaintext auth-ticket layouts gbe_fork emits. Real
// Steam tickets keep the account id inside an encrypted blob, so they report
// false; that is also what distinguishes an official client from an emulated
// one.
func SteamIDFromToken(token string) (uint64, bool) {
	token = strings.TrimSpace(token)
	for _, encoding := range steamTokenEncodings {
		data, err := encoding.DecodeString(token)
		if err != nil {
			continue
		}
		if steamID, ok := gbeAuthTicketSteamID(data); ok {
			return steamID, true
		}
	}
	return 0, false
}

func gbeAuthTicketSteamID(data []byte) (uint64, bool) {
	if len(data) == gbeTokenOldSize &&
		binary.LittleEndian.Uint32(data[0:4]) == gbeTokenOldHeader {
		return individualSteamID(binary.LittleEndian.Uint64(
			data[gbeTokenOldSteamIDAt : gbeTokenOldSteamIDAt+8]))
	}
	// The ticket blob is preceded by its own length. Depending on the settings
	// that length sits at the start, after the GC blob, or after the GC and
	// session blobs.
	for _, lengthOffset := range []int{
		0,
		gbeTokenGCSize + 4,
		gbeTokenGCSize + gbeTokenSessionSize + 4,
	} {
		if steamID, ok := gbeNewAuthTicketSteamID(data, lengthOffset); ok {
			return steamID, true
		}
	}
	return 0, false
}

// gbeNewAuthTicketSteamID reads the current layout. The length includes its own
// four bytes, the ticket blob starts with the version and the signature is
// appended after the blob, so a valid ticket is exactly that long.
func gbeNewAuthTicketSteamID(data []byte, lengthOffset int) (uint64, bool) {
	if lengthOffset+4 > len(data) {
		return 0, false
	}
	layoutLen := int(binary.LittleEndian.Uint32(data[lengthOffset : lengthOffset+4]))
	if layoutLen < 4+gbeTokenStaticLen {
		return 0, false
	}
	if lengthOffset+layoutLen+gbeTokenSignatureLength > len(data) {
		return 0, false
	}
	blob := lengthOffset + 4
	if binary.LittleEndian.Uint32(data[blob:blob+4]) != gbeTokenVersion {
		return 0, false
	}
	return individualSteamID(binary.LittleEndian.Uint64(data[blob+4 : blob+12]))
}

func individualSteamID(id uint64) (uint64, bool) {
	if !looksLikeIndividualSteamID(id) {
		return 0, false
	}
	return id, true
}
