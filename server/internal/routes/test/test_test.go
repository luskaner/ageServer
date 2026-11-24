package test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
)

func TestAnnounceIncludesSafeMode(t *testing.T) {
	oldData := i.AnnounceMessageData
	oldSafe := i.SafeConfigRuntime
	t.Cleanup(func() {
		i.AnnounceMessageData = oldData
		i.SafeConfigRuntime = oldSafe
	})
	i.AnnounceMessageData = map[string]common.AnnounceMessageData002{
		"age4": {GameTitle: "age4", Version: "2"},
	}
	i.SafeConfigRuntime.Enabled = i.SafeTrue

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test/root", nil)
	Test(rr, req, "age4", common.CertModeModern)

	var data common.AnnounceMessageData002
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("answer is not JSON: %v (%s)", err, rr.Body.String())
	}
	if data.Safe != i.SafeTrue {
		t.Fatalf("Safe = %q, want %q", data.Safe, i.SafeTrue)
	}
	if data.Mode != string(common.CertModeModern) {
		t.Fatalf("Mode = %q, want %q", data.Mode, common.CertModeModern)
	}
}
