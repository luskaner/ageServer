# ageLANServer — agent instructions

These rules apply to the whole repository. A nested `AGENTS.md` such as
`gui/server/config/AGENTS.md` adds details for its own directory and takes
precedence there. `DEVELOPMENT.md` describes the toolchain; this file is the
checklist that must be satisfied before a change is considered done.

## Workspace shape

- `go.work` lists 17 modules and the repository root is **not** a module. Run Go
  commands from a module directory (`server`, `common`, …), never from the root:
  `go build ./...` there fails with "directory prefix . does not contain modules
  listed in go.work".
- Shared code lives in `common/`; `battle-server-manager` also pulls
  `battle-server-broadcast` through a `replace` in `go.work`.
- `.github/workflows/tests.yml` is the contract: on ubuntu-latest and
  windows-latest, for `common`, `launcher-common`, `launcher-config-admin`,
  `launcher-config`, `launcher-agent`, `launcher-config-admin-agent`, `server`,
  `battle-server-manager` and `tools/scripts`, it runs `go vet ./...` and
  `go test ./... -count=1`, plus `-race` for `common` and `server`. Everything
  below is what those jobs do, so run it locally first.
- CI pins Go 1.27.3, the same version as `go.work` and every `go.mod`.

## After every change

Run this in every module you touched, and in the modules that import it (the
workspace compiles shared code through `go.work`, so a `common` change has to
pass in `server` too):

```text
gofmt -l .        # must print nothing; run gofmt -w . when it does
go build ./...
go vet ./...
go test ./... -count=1
```

For `common` and `server`, also run what CI runs with the race detector:

```text
go test ./... -count=1 -race
```

When the change touches platform-specific files (`*_windows.go`, `*_unix.go`,
build-tagged tests), type-check the other target as well:

```text
$env:GOOS='linux'; $env:CGO_ENABLED='0'; go vet ./...
```

Two expected exceptions: `gui/server/config` needs CGO for Wails, so skip it in
that run, and `config-helper` is Windows-only (`main_windows.go` defines `run`,
which its untagged tests call), so it can only be vetted on Windows.

### Keep the tree modern

Go 1.27 ships the modernizers the GoLand report asks for. After Go work, apply
them instead of hand-editing:

```text
go fix -any -atomictypes -embedlit -forvar -mapsloop -minmax -newexpr -rangeint `
  -slicesbackward -slicescontains -slicessort -stringscut -stringscutprefix `
  -stringsseq -unsafefuncs ./...
```

(PowerShell above; on a POSIX shell use `\` continuations.) Then repeat the same
command with `-diff` until it prints nothing: one modernizer can enable another
(a `for i := 0; i < n; i++` becomes `for i := range n`, and that can turn an
inner loop into `for range m`). Run `gofmt` afterwards, and read the result —
a generated hunk can be wrong, e.g. substituting `os.PathSeparator` with the
literal `"\\"`, which compiles on Windows and breaks Linux.

Do not enable `omitzero` (it can change JSON output) or `waitgroupgo` (it
rewrites correct `wg.Add`/`go`/`wg.Done` code that the inspection only guessed
was wrong).

### Benchmarks

There are no `func Benchmark*` functions in the repository today, so no
benchmark suite is part of "done". Add one in the package that owns the hot path
when you change performance-sensitive code in `server` or `common`, and run it
before and after the change:

```text
go test ./... -run '^$' -bench . -benchmem
```

Do not add benchmarks just to have them: `server` and `common` already take
minutes under `-race`.

### GUI configurator

`gui/server/config` is Go + React and has its own instructions in
`gui/server/config/AGENTS.md`; the minimum after a change there is:

```text
go test ./...
cd frontend && npx tsc --noEmit
```

## Inspection reports (`docs/report`)

`docs/report/*.xml` is a snapshot of a GoLand batch inspection, not a CI job.
Treat every finding as a prompt to read the code, not as a defect to silence:

- Fix real defects, and prove the rest is fine before changing it.
- The checks above must stay green. A silenced inspection or a `//nolint` is not
  a fix.
- Reviewed false positives — do not "fix" them again:
  * `config-helper/winepath_windows.go`: `go vet` flags the
    `uintptr → unsafe.Pointer` conversion of the C string Wine returns. It is
    safe there (Wine's heap, never Go memory) and cannot be expressed otherwise
    without obfuscation; the comment in the code states it.
  * `ErrInvalidDialog` in `launcher-common/launcher/errors.go`: unused, but it
    is part of an `iota` block, so removing it renumbers every later exit code.
  * `common/uuid` and `server/internal/models/playfab/data`: mixed receivers are
    required — value receivers for the read-only methods, pointer receivers for
    the mutators.
  * `i.A` (which is `[]any`) composite literals cannot elide the element type.
  * The `CssUnusedSymbol` hits in `gui/server/config/frontend/public/style.css`
    are class names built at runtime (`app-infobar--${severity}`,
    `winui-nav-item`).
- `DuplicatedCode` reports need a real extract-method refactor; that is a
  follow-up, not a mechanical edit.

## Before committing

- `gofmt -l` on the touched modules prints nothing and the checks above pass.
- `git status --short` lists only the files you meant to change: build artifacts
  (`changeme.exe`, `config-helper.exe`, `gui/server/config/bin`) must not be
  staged.
- Commits are GPG-signed in this repository. Create them with `git commit -S`
  (and `commit.gpgsign=true`), then confirm with `git verify-commit HEAD`
  before pushing.
- Push, then check `git status -sb`: the branch line must not report
  `[ahead N]`, and the working tree must not hide changes you forgot to commit.
