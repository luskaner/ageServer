//go:build !windows

package text

import (
	"fmt"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func GetEncoding(encodingType int) (enc encoding.Encoding, err error) {
	switch encodingType {
	case EncodingUTF8:
		enc = unicode.UTF8
	case EncodingUTF8BOM:
		enc = unicode.UTF8BOM
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
