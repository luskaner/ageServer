package text

import "testing"

// Split out of text_test.go: these override the Windows Active Code Page via
// SetGetACP, which only exists on Windows, and they exercise ANSI/ACP decoding,
// which is a Windows concept. Moved verbatim; only the location changed.
func TestGetEncodingANSI_SpecificCodePages(t *testing.T) {
	cases := []struct {
		name string
		cp   uint32
	}{
		{"cp1252", 1252},
		{"cp1250", 1250},
		{"cp1251", 1251},
		{"cp1253", 1253},
		{"cp1254", 1254},
		{"cp1255", 1255},
		{"cp1256", 1256},
		{"cp1257", 1257},
		{"cp1258", 1258},
		{"cp932", 932},
		{"cp949", 949},
		{"cp936", 936},
		{"cp950", 950},
		{"cp437", 437},
		{"cp850", 850},
		{"cp852", 852},
		{"cp855", 855},
		{"cp860", 860},
		{"cp862", 862},
		{"cp863", 863},
		{"cp865", 865},
		{"cp866", 866},
		{"cp28591", 28591},
		{"cp28592", 28592},
		{"cp28595", 28595},
		{"cp28597", 28597},
		{"cp28599", 28599},
		{"cp28605", 28605},
		{"cp54936", 54936},
		{"cp20866", 20866},
		{"cp21866", 21866},
		{"unknown", 99999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer SetGetACP(func() uint32 { return tc.cp })()
			enc, err := GetEncoding(EncodingANSI)
			if err != nil {
				t.Fatalf("GetEncoding(EncodingANSI) with cp%d: %v", tc.cp, err)
			}
			if enc == nil {
				t.Fatal("encoding must not be nil")
			}
		})
	}
}

func TestDecodeInvalidUTF8AfterDecode(t *testing.T) {
	// Unknown ACP -> Nop encoding -> invalid UTF-8 check
	defer SetGetACP(func() uint32 { return 99999 })()
	_, _, err := Decode([]byte{0xFF, 0xFF})
	if err == nil {
		t.Fatal("expected error for invalid UTF-8 after decode")
	}
}

func TestDecodeWithSpecificANSI(t *testing.T) {
	defer SetGetACP(func() uint32 { return 1252 })()
	// Encode "é" in Windows-1252 (0xE9)
	raw := []byte{0xE9, 'c', 'o'}
	got, encType, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode ANSI: %v", err)
	}
	if encType != EncodingANSI {
		t.Fatalf("encType = %d, want EncodingANSI", encType)
	}
	if got != "éco" {
		t.Fatalf("decoded = %q, want %q", got, "éco")
	}
}
