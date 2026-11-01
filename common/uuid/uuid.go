// Source: https://github.com/golang/go/blob/master/src/uuid/uuid.go
// TODO: Update if it changes in the future.
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package uuid provides support for generating and manipulating UUIDs.
//
// See [RFC 9562] for details.
//
// Random components of new UUIDs are generated with a
// cryptographically secure random number generator.
//
// UUIDs may be generated using various algorithms.
// The [New] function returns a new UUID generated using
// an algorithm suitable for most purposes.
//
// [RFC 9562]: https://www.rfc-editor.org/rfc/rfc9562.html
package uuid

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// A UUID is a Universally Unique Identifier as specified in RFC 9562.
//
// UUIDs are comparable, such as with the == operator.
type UUID [16]byte

var rander = rand.Reader

// Parse returns the UUID represented by s.
//
// It accepts strings in the following forms:
//
//	f81d4fae-7dec-11d0-a765-00a0c91e6bf6
//	{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}
//	urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6
//	f81d4fae7dec11d0a76500a0c91e6bf6
//
// Alphabetic characters in the hexadecimal digits may be any case.
func Parse(s string) (UUID, error) {
	var u UUID
	err := u.UnmarshalText([]byte(s))
	return u, err
}

// MustParse returns the UUID represented by s.
//
// It panics if s is not a valid string representation of a UUID as defined by [Parse].
func MustParse(s string) UUID {
	u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// New returns a new UUID.
//
// Programs which do not have a need for a specific UUID generation algorithm should use New.
// At this time, New is equivalent to [NewV4].
func New() UUID {
	return NewV4()
}

// Nil returns the Nil UUID 00000000-0000-0000-0000-000000000000.
//
// The Nil UUID is defined in [Section 5.9 of RFC 9562].
// Note that this is not the same as the Go value nil.
//
// [Section 5.9 of RFC 9562]: https://www.rfc-editor.org/rfc/rfc9562#section-5.9
func Nil() UUID {
	return UUID{}
}

// Max returns the Max UUID ffffffff-ffff-ffff-ffff-ffffffffffff.
//
// The Max UUID is defined in [Section 5.10 of RFC 9562].
//
// [Section 5.10 of RFC 9562]: https://www.rfc-editor.org/rfc/rfc9562#section-5.10
func Max() UUID {
	return UUID{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
}

// String returns the string representation of u.
//
// It uses the lowercase hex-and-dash representation defined in RFC 9562.
func (u UUID) String() string {
	b, _ := u.MarshalText()
	return string(b)
}

// MarshalText implements the [encoding.TextMarshaler] interface.
// The encoding is the same as returned by [UUID.String]
func (u UUID) MarshalText() ([]byte, error) {
	return u.AppendText(make([]byte, 0, 36))
}

// AppendText implements the [encoding.TextAppender] interface.
// The encoding is the same as returned by [UUID.String]
func (u UUID) AppendText(b []byte) ([]byte, error) {
	off := len(b)
	b = append(b, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"...)
	dst := b[off:]
	hex.Encode(dst[0:8], u[0:4])
	hex.Encode(dst[9:13], u[4:6])
	hex.Encode(dst[14:18], u[6:8])
	hex.Encode(dst[19:23], u[8:10])
	hex.Encode(dst[24:36], u[10:16])
	return b, nil
}

var errInvalid = errors.New("invalid uuid")

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
// The UUID is expected in a form accepted by [Parse].
func (u *UUID) UnmarshalText(b []byte) error {
	var dst UUID
	switch len(b) {
	case len("urn:uuid:") + 36:
		// urn:uuid:00000000-0000-0000-0000-000000000000
		b = bytes.TrimPrefix(b, []byte("urn:uuid:"))
	case 2 + 36:
		// {00000000-0000-0000-0000-000000000000}
		b = bytes.TrimPrefix(b, []byte("{"))
		b = bytes.TrimSuffix(b, []byte("}"))
	case 32:
		// 00000000000000000000000000000000
		if _, err := hex.Decode(dst[:], b); err != nil {
			return errInvalid
		}
		*u = dst
		return nil
	}
	// 00000000-0000-0000-0000-000000000000
	if len(b) != 36 {
		return errInvalid
	}
	if b[8] != '-' || b[13] != '-' || b[18] != '-' || b[23] != '-' {
		return errInvalid
	}
	if _, err := hex.Decode(dst[0:4], b[0:8]); err != nil {
		return errInvalid
	}
	if _, err := hex.Decode(dst[4:6], b[9:13]); err != nil {
		return errInvalid
	}
	if _, err := hex.Decode(dst[6:8], b[14:18]); err != nil {
		return errInvalid
	}
	if _, err := hex.Decode(dst[8:10], b[19:23]); err != nil {
		return errInvalid
	}
	if _, err := hex.Decode(dst[10:16], b[24:36]); err != nil {
		return errInvalid
	}
	*u = dst
	return nil
}

// Compare compares the UUID u with v.
// If u is before v, it returns -1.
// If u is after v, it returns +1.
// If they are the same, it returns 0.
//
// Compare uses the big-endian byte order defined in
// [Section 6.11 of RFC 9562] for sorting.
//
// [Section 6.11 of RFC 9562]: https://www.rfc-editor.org/rfc/rfc9562#section-6.11
func (u UUID) Compare(v UUID) int {
	for i := range u {
		if c := cmp.Compare(u[i], v[i]); c != 0 {
			return c
		}
	}
	return 0
}

// NewV4 returns a new version 4 UUID.
//
// Version 4 UUIDs contain 122 bits of random data.
func NewV4() UUID {
	var u UUID
	_, _ = rander.Read(u[:])
	u.setVersion(4)
	u.setVariant(0b10)
	return u
}

func (u *UUID) setVersion(version byte) {
	u[6] = (u[6] & 0b0000_1111) | (version << 4)
}

func (u *UUID) setVariant(variant byte) {
	u[8] = (u[8] & 0b0011_1111) | (variant << 6)
}

// SetRand sets the random number generator to r, which implements io.Reader.
// If r.Read returns an error when the package requests random data then
// a panic will be issued.
//
// Calling SetRand with nil sets the random number generator to the default
// generator.
func SetRand(r io.Reader) {
	if r == nil {
		rander = rand.Reader
		return
	}
	rander = r
}
