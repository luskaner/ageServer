package cmd

import (
	"crypto/x509"
	"testing"
)

// Regression: the package-level state vars were never reset between
// invocations, so a failed/partial run leaked its state into the next one and
// undoSetUp/undoRevert would revert things the new run never did.
func TestRunSetUpResetsStateOnEarlyExit(t *testing.T) {
	addedUserCert = true
	backedUpMetadata = true
	backedUpProfiles = true
	addedGameCert = true

	_, _ = runSetUp([]string{"--this-flag-does-not-exist"})

	if addedUserCert || backedUpMetadata || backedUpProfiles || addedGameCert {
		t.Fatalf("state leaked into new invocation: %v %v %v %v",
			addedUserCert, backedUpMetadata, backedUpProfiles, addedGameCert)
	}
}

func TestRunRevertResetsStateOnEarlyExit(t *testing.T) {
	removedUserCerts = []*x509.Certificate{{}}
	removedCaCerts = []*x509.Certificate{{}}
	restoredMetadata = true
	restoredProfiles = true

	_, _ = runRevert([]string{"--this-flag-does-not-exist"})

	if removedUserCerts != nil || removedCaCerts != nil {
		t.Fatalf("slice state leaked: %v %v", removedUserCerts, removedCaCerts)
	}
	if restoredMetadata || restoredProfiles {
		t.Fatalf("bool state leaked: %v %v", restoredMetadata, restoredProfiles)
	}
}
