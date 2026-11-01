package text

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

var getACPFn = windows.GetACP

// SetGetACP replaces the ANSI code page provider for tests.
func SetGetACP(fn func() uint32) (restore func()) {
	orig := getACPFn
	getACPFn = fn
	return func() { getACPFn = orig }
}

// AnsiToUtf8Decoder returns the encoding corresponding to the Windows ANSI Code Page (ACP).
// Filtered to support code pages present in Windows 7 or higher.
func ansiToUtf8Encoding() encoding.Encoding {
	acp := getACPFn()
	switch acp {
	// Windows-125x series (Standard ANSI in modern Windows)
	case 1250:
		return charmap.Windows1250 // Central Europe
	case 1251:
		return charmap.Windows1251 // Cyrillic
	case 1252:
		return charmap.Windows1252 // Western Europe (Latin 1)
	case 1253:
		return charmap.Windows1253 // Greek
	case 1254:
		return charmap.Windows1254 // Turkish
	case 1255:
		return charmap.Windows1255 // Hebrew
	case 1256:
		return charmap.Windows1256 // Arabic
	case 1257:
		return charmap.Windows1257 // Baltic
	case 1258:
		return charmap.Windows1258 // Vietnamese

	// OEM / DOS Code Pages (Commonly used by CMD console)
	case 437:
		return charmap.CodePage437 // US
	// Missing
	/*case 737:
	return charmap.CodePage737 // Greek*/
	// Missing
	/*case 775:
	return charmap.CodePage775 // Baltic*/
	case 850:
		return charmap.CodePage850 // Multilingual Latin 1
	case 852:
		return charmap.CodePage852 // Latin 2
	case 855:
		return charmap.CodePage855 // Cyrillic
	// Missing
	/*case 857:
	return charmap.CodePage857 // Turkish*/
	case 860:
		return charmap.CodePage860 // Portuguese
	// Missing
	/*case 861:
	return charmap.CodePage861 // Icelandic*/
	case 862:
		return charmap.CodePage862 // Hebrew
	case 863:
		return charmap.CodePage863 // Canadian French
	// Missing
	/*case 864:
	return charmap.CodePage864 // Arabic */
	case 865:
		return charmap.CodePage865 // Nordic
	case 866:
		return charmap.CodePage866 // Russian
	// Missing
	/*case 869:
	return charmap.CodePage869 // Modern Greek */
	// ISO-8859 series
	case 28591:
		return charmap.ISO8859_1
	case 28592:
		return charmap.ISO8859_2
	case 28595:
		return charmap.ISO8859_5
	case 28597:
		return charmap.ISO8859_7
	case 28599:
		return charmap.ISO8859_9
	case 28605:
		return charmap.ISO8859_15
	// East Asian Encodings
	case 932:
		return japanese.ShiftJIS
	case 936:
		return simplifiedchinese.GBK
	case 949:
		return korean.EUCKR
	case 950:
		return traditionalchinese.Big5
	case 54936:
		return simplifiedchinese.GB18030
	// Others
	case 20866:
		return charmap.KOI8R // Russian
	case 21866:
		return charmap.KOI8U // Ukrainian
	default:
		return encoding.Nop
	}
}

func GetEncoding(encodingType int) (enc encoding.Encoding, err error) {
	switch encodingType {
	case EncodingUTF8:
		enc = unicode.UTF8
	case EncodingUTF8BOM:
		enc = unicode.UTF8BOM
	case EncodingANSI:
		enc = ansiToUtf8Encoding()
	case EncodingUTF16LE:
		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case EncodingUTF16LEBOM:
		enc = unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM)
	case EncodingUTF16BE:
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case EncodingUTF16BEBOM:
		enc = unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM)
	default:
		err = fmt.Errorf("unsupported encoding")
	}
	return
}
