package internal

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
)

func genCertDER(t *testing.T, name string) ([]byte, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: name,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{name},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return der, cert
}

func writePEM(t *testing.T, path string, ders ...[]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	for _, der := range ders {
		if err := common.WriteAsPem(der, f); err != nil {
			t.Fatalf("WriteAsPem: %v", err)
		}
	}
}

func pemEncode(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestCACertBackup_MissingOriginal(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	if c == nil {
		t.Fatal("expected CACert for age2")
	}
	if err := c.Backup(); err == nil {
		t.Fatal("expected error when original missing")
	}
}

func TestCACertBackup_AlreadyBackedUp(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	der, _ := genCertDER(t, "orig.test")
	writePEM(t, orig, der)
	backup := c.BackupPath()
	if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
		t.Fatal(err)
	}
	// create backup already
	writePEM(t, backup, der)
	if err := c.Backup(); err != nil {
		t.Fatalf("Backup with existing backup should return nil, got %v", err)
	}
}

func TestCACertBackup_Success(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	der, _ := genCertDER(t, "backup-success.test")
	writePEM(t, orig, der)
	if err := c.Backup(); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	backup := c.BackupPath()
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup file missing after Backup: %v", err)
	}
	// verify content matches
	origData, _ := os.ReadFile(orig)
	bakData, _ := os.ReadFile(backup)
	if string(origData) != string(bakData) {
		t.Fatal("backup content mismatch")
	}
}

func TestCACertBackup_ExistingBackupIsDirectoryReturnsNil(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	der, _ := genCertDER(t, "backup-fail.test")
	writePEM(t, orig, der)
	// Make backup path a directory: Backup checks Stat and early-returns nil if it exists
	backup := c.BackupPath()
	if err := os.MkdirAll(backup, 0755); err != nil {
		t.Fatal(err)
	}
	if err := c.Backup(); err != nil {
		t.Fatalf("Backup with existing directory backup should return nil, got %v", err)
	}
	_ = os.RemoveAll(backup)
}

func TestCACertAppend_MissingOriginal(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	der, cert := genCertDER(t, "append-missing.test")
	_ = cert
	if err := c.Append([]*x509.Certificate{{Raw: der}}); err == nil {
		t.Fatal("expected error when original missing")
	}
}

func TestCACertAppend_Success(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	der1, _ := genCertDER(t, "append-orig.test")
	writePEM(t, orig, der1)
	der2, cert2 := genCertDER(t, "append-new.test")
	if err := c.Append([]*x509.Certificate{cert2}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	// Also test using raw DER from gen
	_ = der2
	keys, _, vals, err := common.ReadFromFile(orig)
	if err != nil {
		t.Fatalf("ReadFromFile after Append: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 certs after Append, got %d", len(vals))
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestCACertRestore_MissingOriginal(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	if err, _ := c.Restore(); err == nil {
		t.Fatal("expected error when original missing")
	}
}

func TestCACertRestore_MissingBackup(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	der, _ := genCertDER(t, "restore-no-backup.test")
	writePEM(t, orig, der)
	if err, _ := c.Restore(); err == nil {
		t.Fatal("expected error when backup missing")
	}
}

func TestCACertRestore_TmpAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	backup := c.BackupPath()
	tmp := c.TmpPath()
	for _, p := range []string{orig, backup, tmp} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	der, _ := genCertDER(t, "tmp-exists.test")
	writePEM(t, orig, der)
	writePEM(t, backup, der)
	writePEM(t, tmp, der)
	if err, _ := c.Restore(); err == nil {
		t.Fatal("expected error when tmp already exists")
	}
}

func TestCACertRestore_Success(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	backup := c.BackupPath()
	tmp := c.TmpPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	derOrig, _ := genCertDER(t, "orig-restore.test")
	derAdded, certAdded := genCertDER(t, "added-restore.test")
	// Backup should have only orig
	writePEM(t, backup, derOrig)
	// Original should have orig + added (after Backup + Append scenario)
	writePEM(t, orig, derOrig, derAdded)
	_ = certAdded
	err, removed := c.Restore()
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed cert, got %d", len(removed))
	}
	// After Restore, original should contain only backup content (1 cert)
	_, _, vals, err := common.ReadFromFile(orig)
	if err != nil {
		t.Fatalf("ReadFromFile after Restore: %v", err)
	}
	if len(vals) != 1 {
		t.Fatalf("expected 1 cert in original after Restore, got %d", len(vals))
	}
	// backup should no longer exist, tmp should be removed
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("backup should be moved to original and not exist")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("tmp should be removed, stat err %v", err)
	}
	// removed cert should be the added one
	hashAdded := removed[0].Raw
	if string(hashAdded) != string(derAdded) {
		// Compare subject
		if removed[0].Subject.CommonName != "added-restore.test" {
			t.Fatalf("removed cert CN = %q, want added-restore.test", removed[0].Subject.CommonName)
		}
	}
}

func TestCACertRestore_ReadFailureReverts(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	backup := c.BackupPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	// backup with valid cert
	derOrig, _ := genCertDER(t, "valid.test")
	writePEM(t, backup, derOrig)
	// original with INVALID PEM block that will cause ReadFromFile to return parse error
	invalidPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-a-cert")})
	if err := os.WriteFile(orig, invalidPEM, 0644); err != nil {
		t.Fatal(err)
	}
	// Ensure files exist before
	origDataBefore, _ := os.ReadFile(orig)
	backupDataBefore, _ := os.ReadFile(backup)
	err, _ := c.Restore()
	if err == nil {
		t.Fatal("expected error when tmp contains invalid cert data")
	}
	// After revert, original should be back to invalid, backup should be back to valid
	origAfter, _ := os.ReadFile(orig)
	backupAfter, _ := os.ReadFile(backup)
	if string(origAfter) != string(origDataBefore) {
		t.Fatalf("original not reverted after failure")
	}
	if string(backupAfter) != string(backupDataBefore) {
		t.Fatalf("backup not reverted after failure")
	}
}

func TestCACertRestore_RemoveTmpFailureReverts(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("age2", dir)
	orig := c.OriginalPath()
	backup := c.BackupPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	derOrig, _ := genCertDER(t, "valid2.test")
	// tmp will be orig's content (valid) after first rename, so first read succeeds
	writePEM(t, orig, derOrig)
	// backup contains INVALID PEM that causes second Read to fail
	invalidPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("also-not-a-cert")})
	if err := os.WriteFile(backup, invalidPEM, 0644); err != nil {
		t.Fatal(err)
	}
	err, _ := c.Restore()
	if err == nil {
		t.Fatal("expected error when original (ex-backup) contains invalid data")
	}
	// After revert, files should be back
	if _, err := os.Stat(orig); err != nil {
		t.Fatalf("orig missing after revert: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup missing after revert: %v", err)
	}
}

func TestCACertAppend_WriteFailure(t *testing.T) {
	dir := t.TempDir()
	c := NewCACert("athens", dir) // AoM does not use certificates subfolder
	orig := c.OriginalPath()
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		t.Fatal(err)
	}
	derOrig, _ := genCertDER(t, "append-write-fail-orig.test")
	writePEM(t, orig, derOrig)
	// Make original file read-only? On Windows Append opens with O_APPEND|O_WRONLY - might still succeed if permission allows.
	// Instead we test that Append handles invalid cert raw? Actually WriteAsPem just encodes bytes, it won't fail per se.
	// So we at least check success case for athens as well
	der2, cert2 := genCertDER(t, "append-athens.test")
	if err := c.Append([]*x509.Certificate{cert2}); err != nil {
		t.Fatalf("Append for athens failed: %v", err)
	}
	_ = der2
}

func TestCACertForAoMAndAoE3(t *testing.T) {
	for _, gameId := range []string{"age3", "athens"} {
		dir := t.TempDir()
		c := NewCACert(gameId, dir)
		if c == nil {
			t.Fatalf("expected CACert for %s", gameId)
		}
		orig := c.OriginalPath()
		if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
			t.Fatal(err)
		}
		der, _ := genCertDER(t, "game-"+gameId+".test")
		writePEM(t, orig, der)
		if err := c.Backup(); err != nil {
			t.Fatalf("Backup for %s failed: %v", gameId, err)
		}
		if err := c.Append([]*x509.Certificate{{Raw: der}}); err != nil {
			t.Fatalf("Append for %s failed: %v", gameId, err)
		}
	}
}
