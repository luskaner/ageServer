package party

import (
	"regexp"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
)

func TestReportMatchResponseHasReplaySlotsInPlace(t *testing.T) {
	descriptor := replayDescriptor(42, "http://host/cloudfiles/replays/42/0/x.gz", "host", "/cloudfiles/replays/42/0/x.gz", false, "sig=a&sp=cw")
	response := reportMatchResponse(0, []i.A{descriptor})
	if len(response) != 16 {
		t.Fatalf("response has %d fields, want 16", len(response))
	}
	if response[0] != 0 {
		t.Fatalf("result code = %v, want 0", response[0])
	}
	if got, ok := response[2].(i.A); !ok || len(got) != 6 || got[0] != int64(42) || got[4] != 0 {
		t.Fatalf("slot 2 = %#v", response[2])
	}
	if got, ok := response[3].(i.A); !ok || got[0] != -1 {
		t.Fatalf("slot 3 = %#v, want an empty descriptor", response[3])
	}

	empty := reportMatchResponse(0, nil)
	if len(empty) != 16 {
		t.Fatalf("empty response has %d fields, want 16", len(empty))
	}
	if got, ok := empty[2].(i.A); !ok || got[0] != -1 {
		t.Fatalf("empty slot 2 = %#v", empty[2])
	}
}

func TestSinglePlayerResponseShape(t *testing.T) {
	descriptor := replayDescriptor(7, "url", "host", "path", true, "sig")
	created := singlePlayerResponse(0, 1234, "key", i.A{1}, nil)
	if len(created) != 22 {
		t.Fatalf("creation response has %d fields, want 22", len(created))
	}
	if created[1] != int64(1234) || created[2] != "key" {
		t.Fatalf("creation match fields = %v, %v", created[1], created[2])
	}
	if created[12] != nil {
		t.Fatalf("creation index 12 = %v, want null", created[12])
	}
	if got, ok := created[13].(i.A); !ok || got[0] != -1 {
		t.Fatalf("creation slot 13 = %#v", created[13])
	}

	reported := singlePlayerResponse(0, 0, nil, i.A{1}, []i.A{descriptor, descriptor})
	if len(reported) != 22 {
		t.Fatalf("report response has %d fields, want 22", len(reported))
	}
	if reported[12] != "" {
		t.Fatalf("report index 12 = %v, want an empty string", reported[12])
	}
	for _, index := range []int{13, 14} {
		got, ok := reported[index].(i.A)
		if !ok || len(got) != 6 || got[4] != 1 {
			t.Fatalf("report slot %d = %#v", index, reported[index])
		}
	}
}

func TestReplayKeyIsDeterministicAndDistinctPerKind(t *testing.T) {
	first := replayKey(200058826, 0)
	if first != replayKey(200058826, 0) {
		t.Fatal("replayKey is not deterministic")
	}
	if first == replayKey(200058826, 1) {
		t.Fatal("different kinds share the same key")
	}
	pattern := regexp.MustCompile(`^replays/200058826/0/[0-9a-f]{32}\.gz$`)
	if !pattern.MatchString(first) {
		t.Fatalf("replayKey = %q", first)
	}
}
