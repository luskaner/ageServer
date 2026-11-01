package text

import (
	"testing"
)

// A single high byte is not valid UTF-8 and too short for heuristics:
// it must be classified as Other and rejected by Decode/GetEncoding.
func TestEncodingOtherSingleHighByte(t *testing.T) {
	buf := []byte{0xE9}
	if got := Encoding(buf); got != EncodingOther {
		t.Fatalf("Encoding = %d, want EncodingOther", got)
	}
	if _, _, err := Decode(buf); err == nil {
		t.Fatal("Decode must fail for EncodingOther")
	}
	if _, err := GetEncoding(EncodingOther); err == nil {
		t.Fatal("GetEncoding(EncodingOther) must error")
	}
}

func TestEncodingUTF16NoBomBothEndians(t *testing.T) {
	le := []byte{'h', 0x00, 'i', 0x00}
	be := []byte{0x00, 'h', 0x00, 'i'}
	if got := Encoding(le); got != EncodingUTF16LE {
		t.Errorf("LE heuristic = %d", got)
	}
	if got := Encoding(be); got != EncodingUTF16BE {
		t.Errorf("BE heuristic = %d", got)
	}
}

func TestGetEncodingAllNamedConstants(t *testing.T) {
	for _, encType := range []int{
		EncodingUTF16LEBOM,
		EncodingUTF16LE,
		EncodingUTF16BEBOM,
		EncodingUTF16BE,
		EncodingUTF8,
		EncodingUTF8BOM,
	} {
		enc, err := GetEncoding(encType)
		if err != nil || enc == nil {
			t.Errorf("GetEncoding(%d) = %v, %v", encType, enc, err)
		}
	}
}

func TestDecodeUtf8BomStripsPrefix(t *testing.T) {
	const text = "plain"
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...)
	got, encType, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if encType != EncodingUTF8BOM {
		t.Fatalf("encType = %d", encType)
	}
	if got != text {
		t.Fatalf("got %q, want %q (BOM must be stripped)", got, text)
	}
}
