package ops

import (
	"testing"
)

func TestParseCommandArgsSubstitutesVariables(t *testing.T) {
	args, err := ParseCommandArgs(
		[]string{"--id", "{Id}", "-e", "{Game}"},
		map[string]string{"Id": "abc-123", "Game": "age2"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--id", "abc-123", "-e", "age2"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseCommandArgsWordSplitting(t *testing.T) {
	args, err := ParseCommandArgs([]string{"run", "--now"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "run" || args[1] != "--now" {
		t.Fatalf("args = %v", args)
	}
}

func TestResolveIsolateValue(t *testing.T) {
	cases := []struct {
		value            string
		officialLauncher bool
		want             bool
	}{
		{"true", false, true},
		{"false", true, false},
		{"required", true, true},
		{"required", false, false},
		{"garbage", true, false},
		{"", false, false},
	}
	for _, tc := range cases {
		if got := ResolveIsolateValue(tc.value, tc.officialLauncher); got != tc.want {
			t.Errorf("ResolveIsolateValue(%q,%v) = %v, want %v", tc.value, tc.officialLauncher, got, tc.want)
		}
	}
}
