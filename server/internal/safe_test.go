package internal

import "testing"

func TestParseSafeMode(t *testing.T) {
	cases := map[string]string{
		"":      SafeAuto,
		"auto":  SafeAuto,
		"AUTO":  SafeAuto,
		" true": SafeTrue,
		"TRUE":  SafeTrue,
		"1":     SafeTrue,
		"false": SafeFalse,
		"FALSE": SafeFalse,
		"0":     SafeFalse,
	}
	for value, want := range cases {
		got, err := ParseSafeMode(value)
		if err != nil {
			t.Errorf("ParseSafeMode(%q): %s", value, err)
			continue
		}
		if got != want {
			t.Errorf("ParseSafeMode(%q) = %q, want %q", value, got, want)
		}
	}
	if _, err := ParseSafeMode("sometimes"); err == nil {
		t.Fatal("ParseSafeMode accepted an invalid value")
	}
}

func TestSafeEnabled(t *testing.T) {
	old := SafeConfigRuntime
	t.Cleanup(func() { SafeConfigRuntime = old })

	cases := []struct {
		name       string
		mode       string
		official   bool
		exceptions []string
		userId     uint64
		want       bool
	}{
		{"auto emulated", SafeAuto, false, nil, 42, false},
		{"auto official", SafeAuto, true, nil, 42, true},
		{"true emulated", SafeTrue, false, nil, 42, true},
		{"false official", SafeFalse, true, nil, 42, false},
		{"auto official exception", SafeAuto, true, []string{"42"}, 42, false},
		{"auto emulated exception", SafeAuto, false, []string{"42"}, 42, true},
		{"exception does not match", SafeAuto, true, []string{"43"}, 42, true},
		{"zero id never matches", SafeAuto, false, []string{"0"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SafeConfigRuntime.Enabled = tc.mode
			SafeConfigRuntime.PlatformUserIdExceptions = tc.exceptions
			if got := SafeEnabled(tc.official, tc.userId); got != tc.want {
				t.Fatalf("SafeEnabled(%v, %d) = %v, want %v", tc.official, tc.userId, got, tc.want)
			}
		})
	}
}

func TestPlatformUserIdInExceptions(t *testing.T) {
	exceptions := []string{"76561198000000001", "42"}
	if !PlatformUserIdInExceptions(42, exceptions) {
		t.Fatal("42 was not found")
	}
	if !PlatformUserIdInExceptions(76561198000000001, exceptions) {
		t.Fatal("the Steam ID was not found")
	}
	if PlatformUserIdInExceptions(43, exceptions) {
		t.Fatal("43 matched")
	}
	if PlatformUserIdInExceptions(0, []string{"0"}) {
		t.Fatal("zero matched")
	}
}
