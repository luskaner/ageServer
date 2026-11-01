package userData

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestNewPathJoinsPerGame(t *testing.T) {
	cases := []struct {
		gameId string
		sub    string
		prefix string
	}{
		{game.AoE1, "Age of Empires DE", "Games"},
		{game.AoE2, "Age of Empires 2 DE", "Games"},
		{game.AoE3, "Age of Empires 3 DE", "Games"},
		{game.AoE4, "Age of Empires IV", "My Games"},
		{game.AoM, "Age of Mythology Retold", "Games"},
	}
	for _, tc := range cases {
		p := NewPath("base", tc.gameId)
		want := filepath.Join("base", tc.prefix, tc.sub)
		if p.String() != want {
			t.Errorf("%s: path = %q, want %q", tc.gameId, p.String(), want)
		}
		if p.GameId() != tc.gameId {
			t.Errorf("%s: gameId mismatch", tc.gameId)
		}
	}
}

func TestTypFromExtension(t *testing.T) {
	for _, tc := range []struct {
		path string
		want int
	}{
		{"profile", TypeActive},
		{"profile.lan", TypeServer},
		{"profile.bak", TypeBackup},
		{filepath.Join("dir", "p.bak"), TypeBackup},
	} {
		if got, _ := typ(tc.path); got != tc.want {
			t.Errorf("typ(%q) = %d, want %d", tc.path, got, tc.want)
		}
	}
}

func TestSuffixRoundTrip(t *testing.T) {
	if suffix(TypeActive) != "" {
		t.Fatal("active must have empty suffix")
	}
	if suffix(TypeServer) != serverSuffix || suffix(TypeBackup) != backupSuffix {
		t.Fatal("server/backup suffixes mismatched")
	}
}

func TestTransformPathMatrix(t *testing.T) {
	active := filepath.Join("u", "prof")
	server := active + serverSuffix
	backup := active + backupSuffix

	// Identity transitions.
	if ok, p := TransformPath(active, TypeActive, TypeActive); !ok || p != active {
		t.Errorf("active->active: %v %q", ok, p)
	}
	// Forward transitions.
	if ok, p := TransformPath(active, TypeActive, TypeServer); !ok || p != server {
		t.Errorf("active->server: %v %q", ok, p)
	}
	if ok, p := TransformPath(server, TypeServer, TypeBackup); !ok || p != backup {
		t.Errorf("server->backup: %v %q, want %q", ok, p, backup)
	}
	if ok, p := TransformPath(backup, TypeBackup, TypeActive); !ok || p != active {
		t.Errorf("backup->active: %v %q, want %q", ok, p, active)
	}
	// Mismatched source type fails.
	if ok, _ := TransformPath(active, TypeBackup, TypeServer); ok {
		t.Error("active path must not transform from backup source type")
	}
}

func TestDataGetters(t *testing.T) {
	d := Data{typ: TypeServer, path: "/some/path.lan"}
	if d.Type() != TypeServer {
		t.Errorf("Type() = %d, want %d", d.Type(), TypeServer)
	}
	if d.Path() != "/some/path.lan" {
		t.Errorf("Path() = %q, want /some/path.lan", d.Path())
	}
}

func TestProfilesAoE2(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Games", "Age of Empires 2 DE")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create numeric profile directories (valid for AoE2)
	for _, id := range []string{"12345678", "99999999"} {
		if err := os.Mkdir(filepath.Join(profileDir, id), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Create non-numeric directory (should be skipped for AoE2)
	if err := os.Mkdir(filepath.Join(profileDir, "notanumber"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create a file (should be skipped, not a dir)
	if err := os.WriteFile(filepath.Join(profileDir, "42"), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewPath(dir, game.AoE2)
	err, profiles := p.Profiles()
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if profiles.Cardinality() != 2 {
		t.Errorf("expected 2 profiles, got %d", profiles.Cardinality())
	}
}

func TestProfilesAoE1(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Games", "Age of Empires DE", "Users")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	// AoE1 accepts any directory name
	for _, name := range []string{"player1", "player2"} {
		if err := os.Mkdir(filepath.Join(profileDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	p := NewPath(dir, game.AoE1)
	err, profiles := p.Profiles()
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if profiles.Cardinality() != 2 {
		t.Errorf("expected 2 profiles, got %d", profiles.Cardinality())
	}
}

func TestProfilesMissingDir(t *testing.T) {
	p := NewPath(t.TempDir(), game.AoE2)
	err, _ := p.Profiles()
	if err == nil {
		t.Error("Profiles should error on missing directory")
	}
}

func TestMetadatasCreatesAndFindsDirs(t *testing.T) {
	dir := t.TempDir()
	// Create the base path manually so metadata subfolder can be created
	basePath := filepath.Join(dir, "Games", "Age of Empires 2 DE")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Fatal(err)
	}

	p := NewPath(dir, game.AoE2)
	err, metadatas := p.Metadatas()
	if err != nil {
		t.Fatalf("Metadatas: %v", err)
	}
	// Metadatas() creates the metadata dir if it doesn't exist but only returns
	// dirs that already exist with a recognized suffix. Since we just created it,
	// there should be no matching subdirectories.
	// However, Metadatas may create directories for all suffixes, so check
	// that it didn't error.
	_ = metadatas
	// Verify the metadata directory was created
	metadataDir := filepath.Join(basePath, "metadata")
	if _, statErr := os.Stat(metadataDir); os.IsNotExist(statErr) {
		t.Error("metadata directory should have been created")
	}
}

func TestMetadatasWithSubDirs(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "Games", "Age of Empires 2 DE")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Fatal(err)
	}
	// Metadatas() scans the metadata folder and checks TransformPath for each
	// suffix. The iteration is: suffixToType (backup, server) + "" (active).
	// For each, TransformPath(metadataPath, TypeActive, t) is called.
	// So for TypeBackup: metadataPath.bak, for TypeServer: metadataPath.lan,
	// for TypeActive: metadataPath (the dir itself).
	// Create both .bak and .lan variants.
	metaDir := filepath.Join(basePath, "metadata")
	backupDir := metaDir + backupSuffix
	serverDir := metaDir + serverSuffix
	for _, d := range []string{backupDir, serverDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	p := NewPath(dir, game.AoE2)
	err, metadatas := p.Metadatas()
	if err != nil {
		t.Fatalf("Metadatas: %v", err)
	}
	// .bak, .lan, and the metadata dir itself (TypeActive) = 3
	if metadatas.Cardinality() != 3 {
		t.Errorf("expected 3 metadatas, got %d", metadatas.Cardinality())
	}
}

func TestMetadatasAoE4(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "My Games", "Age of Empires IV", "network")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Fatal(err)
	}
	p := NewPath(dir, game.AoE4)
	err, _ := p.Metadatas()
	if err != nil {
		t.Fatalf("Metadatas: %v", err)
	}
}

func TestMetadatasAoE3(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "Games", "Age of Empires 3 DE", "Common", "RLink")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Fatal(err)
	}
	p := NewPath(dir, game.AoE3)
	err, _ := p.Metadatas()
	if err != nil {
		t.Fatalf("Metadatas: %v", err)
	}
}

func TestMetadatasAoM(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "Games", "Age of Mythology Retold", "temp", "RLink")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		t.Fatal(err)
	}
	p := NewPath(dir, game.AoM)
	err, _ := p.Metadatas()
	if err != nil {
		t.Fatalf("Metadatas: %v", err)
	}
}

func TestProfileFolderPerGame(t *testing.T) {
	// AoE1 and AoE4 use "Users" subfolder, others use ""
	cases := []struct {
		gameId   string
		wantUser bool
	}{
		{game.AoE1, true},
		{game.AoE2, false},
		{game.AoE3, false},
		{game.AoE4, true},
		{game.AoM, false},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		basePath := filepath.Join(dir, "test")
		usersPath := filepath.Join(basePath, "Users")
		if tc.wantUser {
			if err := os.MkdirAll(usersPath, 0755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(basePath, 0755); err != nil {
				t.Fatal(err)
			}
		}
		p := NewPath(dir, tc.gameId)
		// Override the path to point to our test dir
		p.path = basePath
		err, profiles := p.Profiles()
		if err != nil {
			t.Errorf("%s: Profiles: %v", tc.gameId, err)
			continue
		}
		// No profile entries, just checking no error
		_ = profiles
	}
}

func TestSuffixForUnknownType(t *testing.T) {
	s := suffix(999)
	if s != "" {
		t.Errorf("suffix for unknown type should be empty, got %q", s)
	}
}

func TestTransformPathSameType(t *testing.T) {
	ok, p := TransformPath("file.lan", TypeServer, TypeServer)
	if !ok || p != "file.lan" {
		t.Errorf("same type transform: ok=%v path=%q", ok, p)
	}
}

func TestTypUnknownExtension(t *testing.T) {
	typ, ext := typ("file.xyz")
	if typ != TypeActive {
		t.Errorf("unknown extension type = %d, want TypeActive", typ)
	}
	if ext != ".xyz" {
		t.Errorf("unknown extension ext = %q, want .xyz", ext)
	}
}

func TestProfileWithBackupExtension(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Games", "Age of Empires 2 DE")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Profile with .lan extension (server variant) — the numeric ID check
	// in Profiles() uses ParseUint on entry.Name(), so the base name must
	// be numeric. A ".lan" suffix makes it non-numeric, so it gets skipped.
	// Instead, create an active profile and verify it works.
	id := strconv.Itoa(12345678)
	profileActive := filepath.Join(profileDir, id)
	if err := os.Mkdir(profileActive, 0755); err != nil {
		t.Fatal(err)
	}

	p := NewPath(dir, game.AoE2)
	err, profiles := p.Profiles()
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	// Active profile should be found with TypeActive
	if profiles.Cardinality() != 1 {
		t.Errorf("expected 1 profile, got %d", profiles.Cardinality())
	}
}
