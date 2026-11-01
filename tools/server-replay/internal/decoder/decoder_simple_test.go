package decoder

import (
	"os"
	"testing"
)

func TestDecodeInvalidJSON(t *testing.T) {
	tmp, _ := os.CreateTemp(t.TempDir(), "log*.json")
	tmp.Write([]byte("not json\n"))
	tmp.Close()
	f, _ := os.Open(tmp.Name())
	err := Decode(f)
	f.Close()
	if err == nil {
		t.Fatal("should fail on invalid json")
	}
}

func TestDecodeUnknownType(t *testing.T) {
	tmp, _ := os.CreateTemp(t.TempDir(), "log*.json")
	tmp.Write([]byte(`{"type":"unknown"}` + "\n"))
	tmp.Close()
	f, _ := os.Open(tmp.Name())
	err := Decode(f)
	f.Close()
	if err == nil {
		t.Fatal("should fail on unknown type")
	}
	// Check error message
	if err.Error() != "unrecognized message type: unknown" {
		t.Fatalf("wrong error %v", err)
	}
}

func TestDecodeEmpty(t *testing.T) {
	tmp, _ := os.CreateTemp(t.TempDir(), "log*.json")
	tmp.Close()
	f, _ := os.Open(tmp.Name())
	if err := Decode(f); err != nil {
		t.Fatalf("empty should not error, got %v", err)
	}
	f.Close()
}

func TestDecodeRequestInvalidJSON(t *testing.T) {
	tmp, _ := os.CreateTemp(t.TempDir(), "log*.json")
	// Write a valid MessageType for request, but invalid request JSON (missing required fields, but json unmarshal will still succeed? Let's make it invalid json for request)
	tmp.Write([]byte(`{"type":"request"}` + "\n"))
	tmp.Write([]byte(`{"type":"request","in":{bad json}` + "\n"))
	tmp.Close()
	f, _ := os.Open(tmp.Name())
	err := Decode(f)
	f.Close()
	if err == nil {
		t.Fatal("should fail on invalid request json")
	}
}
