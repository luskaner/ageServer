//go:build windows

package main

import (
	"errors"
	"testing"
)

func TestRun_ConfigPathTrueAndFalse(t *testing.T) {
	origSteamAlt := steamConfigPathAltFn
	origSteam := steamConfigPathFn
	origWinToUnix := windowsToUnixPathFn
	origPrint := fmtPrintFn
	defer func() {
		steamConfigPathAltFn = origSteamAlt
		steamConfigPathFn = origSteam
		windowsToUnixPathFn = origWinToUnix
		fmtPrintFn = origPrint
	}()

	steamConfigPathAltFn = func() string { return `C:\Steam\config_alt` }
	steamConfigPathFn = func() string { return `C:\Steam\config` }
	windowsToUnixPathFn = func(path string) (string, error) {
		if path == `C:\Steam\config_alt` {
			return "/unix/alt", nil
		}
		if path == `C:\Steam\config` {
			return "/unix/config", nil
		}
		t.Fatalf("unexpected path %q", path)
		return "", nil
	}
	printed := ""
	fmtPrintFn = func(a ...any) (int, error) {
		printed = a[0].(string)
		return len(printed), nil
	}

	if err := run([]string{"config-helper", "configPath", "true"}); err != nil {
		t.Fatalf("configPath true failed: %v", err)
	}
	if printed != "/unix/alt" {
		t.Fatalf("printed %q want /unix/alt", printed)
	}
	printed = ""
	if err := run([]string{"config-helper", "configPath", "false"}); err != nil {
		t.Fatalf("configPath false failed: %v", err)
	}
	if printed != "/unix/config" {
		t.Fatalf("printed %q want /unix/config", printed)
	}
	// Any non-true should go to ConfigPath (original code uses == "true" else)
	printed = ""
	if err := run([]string{"config-helper", "configPath", "maybe"}); err != nil {
		t.Fatalf("configPath maybe failed: %v", err)
	}
	if printed != "/unix/config" {
		t.Fatalf("printed %q want /unix/config for maybe", printed)
	}
}

func TestRun_UserProfilePath(t *testing.T) {
	origGame := gameUserProfilePathFn
	origWinToUnix := windowsToUnixPathFn
	origPrint := fmtPrintFn
	defer func() {
		gameUserProfilePathFn = origGame
		windowsToUnixPathFn = origWinToUnix
		fmtPrintFn = origPrint
	}()

	gameUserProfilePathFn = func(profile string) string {
		return `C:\Users\` + profile
	}
	windowsToUnixPathFn = func(path string) (string, error) {
		return "/unix/" + path, nil
	}
	printed := ""
	fmtPrintFn = func(a ...any) (int, error) {
		printed = a[0].(string)
		return len(printed), nil
	}

	if err := run([]string{"config-helper", "userProfilePath", "testProfile"}); err != nil {
		t.Fatalf("userProfilePath failed: %v", err)
	}
	if printed == "" {
		t.Fatal("printed empty")
	}
}

func TestRun_WindowsToUnixPathSuccess(t *testing.T) {
	origWinToUnix := windowsToUnixPathFn
	origPrint := fmtPrintFn
	defer func() {
		windowsToUnixPathFn = origWinToUnix
		fmtPrintFn = origPrint
	}()

	windowsToUnixPathFn = func(path string) (string, error) {
		if path != `C:\test` {
			t.Fatalf("path %q", path)
		}
		return "/unix/test", nil
	}
	printed := ""
	fmtPrintFn = func(a ...any) (int, error) {
		printed = a[0].(string)
		return len(printed), nil
	}

	if err := run([]string{"config-helper", "windowsToUnixPath", `C:\test`}); err != nil {
		t.Fatalf("windowsToUnixPath failed: %v", err)
	}
	if printed != "/unix/test" {
		t.Fatalf("printed %q", printed)
	}
}

func TestRun_WindowsToUnixPathFailure(t *testing.T) {
	origWinToUnix := windowsToUnixPathFn
	defer func() { windowsToUnixPathFn = origWinToUnix }()

	windowsToUnixPathFn = func(string) (string, error) {
		return "", errors.New("wine fail")
	}
	if err := run([]string{"config-helper", "windowsToUnixPath", `C:\test`}); err == nil {
		t.Fatal("expected wine fail")
	}
}

func TestConvertAndPrint_SuccessAndFailure(t *testing.T) {
	origWinToUnix := windowsToUnixPathFn
	origPrint := fmtPrintFn
	defer func() {
		windowsToUnixPathFn = origWinToUnix
		fmtPrintFn = origPrint
	}()

	windowsToUnixPathFn = func(string) (string, error) {
		return "/unix/ok", nil
	}
	printed := ""
	fmtPrintFn = func(a ...any) (int, error) {
		printed = a[0].(string)
		return len(printed), nil
	}
	if err := convertAndPrint(`C:\ok`); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if printed != "/unix/ok" {
		t.Fatalf("printed %q", printed)
	}

	windowsToUnixPathFn = func(string) (string, error) {
		return "", errors.New("fail")
	}
	if err := convertAndPrint(`C:\fail`); err == nil {
		t.Fatal("expected fail")
	}
}

func TestRun_ConvertAndPrintIntegration(t *testing.T) {
	// Test that run for configPath uses convertAndPrint which uses windowsToUnixPathFn
	origSteam := steamConfigPathFn
	origWinToUnix := windowsToUnixPathFn
	origPrint := fmtPrintFn
	defer func() {
		steamConfigPathFn = origSteam
		windowsToUnixPathFn = origWinToUnix
		fmtPrintFn = origPrint
	}()

	steamConfigPathFn = func() string { return `C:\config` }
	windowsToUnixPathFn = func(string) (string, error) { return "/unix/config", nil }
	calledPrint := false
	fmtPrintFn = func(a ...any) (int, error) {
		calledPrint = true
		return 0, nil
	}
	if err := run([]string{"config-helper", "configPath", "false"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !calledPrint {
		t.Error("fmtPrint not called")
	}
}
