package text

import (
	"errors"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

func TestEncodingDetection(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want int
	}{
		{"empty is utf8", nil, EncodingUTF8},
		{"ascii", []byte("hello hosts"), EncodingUTF8},
		{"utf8 bom", append([]byte{0xEF, 0xBB, 0xBF}, []byte("data")...), EncodingUTF8BOM},
		{"utf16le bom", []byte{0xFF, 0xFE, 'a', 0x00}, EncodingUTF16LEBOM},
		{"utf16be bom", []byte{0xFE, 0xFF, 0x00, 'a'}, EncodingUTF16BEBOM},
		{"utf16le no bom", []byte{'h', 0x00, 'i', 0x00}, EncodingUTF16LE},
		{"utf16be no bom", []byte{0x00, 'h', 0x00, 'i'}, EncodingUTF16BE},
		{"single ascii byte", []byte{'x'}, EncodingUTF8},
		{"ansi high bytes", []byte{0xE9, 'c', 'o'}, EncodingANSI},
	}
	for _, tc := range cases {
		if got := Encoding(tc.buf); got != tc.want {
			t.Errorf("%s: Encoding = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestDecodeUTF8Variants(t *testing.T) {
	text := "# hosts file\r\n1.2.3.4 example.com\r\n"

	got, encType, err := Decode([]byte(text))
	if err != nil || encType != EncodingUTF8 || got != text {
		t.Fatalf("plain utf8: %q, %d, %v", got, encType, err)
	}

	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...)
	got, encType, err = Decode(bom)
	if err != nil || encType != EncodingUTF8BOM || got != text {
		t.Fatalf("utf8 bom: %q, %d, %v", got, encType, err)
	}
}

func decodeWith(t *testing.T, encType int, src string) string {
	t.Helper()
	enc, err := GetEncoding(encType)
	if err != nil {
		t.Fatalf("GetEncoding(%d): %v", encType, err)
	}
	encoded, err := enc.NewEncoder().Bytes([]byte(src))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(encoded)
}

func TestDecodeUTF16RoundTrip(t *testing.T) {
	const text = "127.0.0.1 example.com"

	for _, encType := range []int{EncodingUTF16LEBOM, EncodingUTF16BEBOM} {
		raw := []byte(decodeWith(t, encType, text))
		got, detected, err := Decode(raw)
		if err != nil {
			t.Errorf("encType %d: Decode error: %v", encType, err)
			continue
		}
		if detected != encType {
			t.Errorf("encType %d: detected %d", encType, detected)
		}
		if got != text {
			t.Errorf("encType %d: decoded %q, want %q", encType, got, text)
		}
	}
}

func TestGetEncodingInvalidErrors(t *testing.T) {
	if _, err := GetEncoding(9999); err == nil {
		t.Fatal("expected error for out-of-range encoding type")
	}
}

func TestDecodeAnsiSmoke(t *testing.T) {
	// On Windows ANSI maps to the active code page (no error); elsewhere it errors.
	got, encType, err := Decode([]byte{0xE9, 'c', 'o'})
	if err != nil {
		return
	}
	if encType != EncodingANSI {
		t.Fatalf("encType = %d, want EncodingANSI", encType)
	}
	if got == "" {
		t.Fatal("ANSI decode produced empty text without error")
	}
}

func TestGetEncodingKnownValuesDoNotError(t *testing.T) {
	for _, encType := range []int{
		EncodingUTF16LEBOM, EncodingUTF16LE,
		EncodingUTF16BEBOM, EncodingUTF16BE,
		EncodingUTF8, EncodingUTF8BOM,
	} {
		if _, err := GetEncoding(encType); err != nil {
			t.Errorf("GetEncoding(%d): %v", encType, err)
		}
	}
}

func TestDecodeUnsupportedEncoding(t *testing.T) {
	// Single invalid byte 0xFF -> EncodingOther -> GetEncoding error
	_, _, err := Decode([]byte{0xFF})
	if err == nil {
		t.Fatal("expected error for unsupported encoding")
	}
}

type errTransformer struct{}

func (errTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	return 0, 0, errors.New("mock decode error")
}
func (errTransformer) Reset() {}

type failEncoding struct{}

func (failEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: errTransformer{}}
}
func (failEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: errTransformer{}}
}

func TestDecodeNewDecoderError(t *testing.T) {
	_ = transform.Nop
	orig := getEncodingFn
	defer func() { getEncodingFn = orig }()
	getEncodingFn = func(int) (encoding.Encoding, error) { return failEncoding{}, nil }
	_, _, err := Decode([]byte("any"))
	if err == nil {
		t.Fatal("expected error from decoder")
	}
	if err.Error() != "mock decode error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEncodingFinalReturn(t *testing.T) {
	orig := utf8ValidFn
	defer func() { utf8ValidFn = orig }()
	utf8ValidFn = func(p []byte) bool { return false }
	// Buffer with n>=2, no BOM, heuristic fails (no null bytes), utf8Valid false, no byte >=0x80
	buf := []byte{'a', 'b'}
	if got := Encoding(buf); got != EncodingOther {
		t.Fatalf("Encoding with mocked utf8Valid false and no high bits = %d, want %d", got, EncodingOther)
	}
}
