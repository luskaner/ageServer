package launcher

import (
	"testing"
)

// A session with no presenter installed must draw nothing.
//
// This is the default a graphical frontend gets before it installs its own, and
// it is also what a frontend that forgets gets. The alternative is a run that
// writes banner, headings and a spinner to whatever stdout happens to be
// attached, which in a window is escape sequences nobody asked for.
func TestSilentPresenterDrawsNothing(t *testing.T) {
	var p Presenter = SilentPresenter{}
	// Every method, so that adding one to the interface and forgetting it here
	// is a compile error rather than a surprise at runtime.
	p.Banner("launcher", "1.2.3")
	p.Section("Execution")
	p.KV(0, "game", "age2")
	p.ApplyOutput("ascii")
	p.ClearProgress()
	if hint := p.Hint(); hint != "" {
		t.Errorf("a presenter that shows nothing has nothing to hint about, got %q", hint)
	}
	if s := p.Start("Looking for the game..."); s == nil {
		t.Error("Start returned nil, which every caller dereferences")
	} else {
		s.Done("Game found on %s.", "Steam")
		s.Fail("Game not found.")
		s.Info("No servers found.")
		s.Stop()
	}
	if bar := p.BeginProgress(); bar == nil {
		t.Error("BeginProgress returned nil, which every caller dereferences")
	} else {
		bar.Set(50)
		bar.Done()
		bar.Fail()
	}
}

// Installing and removing a Presenter is how a frontend hands one over and takes
// it back, so both directions have to work and a removed one must not be sticky.
func TestPresenterSlot(t *testing.T) {
	t.Cleanup(ResetPresenter)
	ResetPresenter()
	if _, ok := ActivePresenter().(SilentPresenter); !ok {
		t.Fatalf("with nothing installed the presenter is %T, want the silent one", ActivePresenter())
	}

	want := &countingPresenter{}
	SetPresenter(want)
	if got := ActivePresenter(); got != Presenter(want) {
		t.Errorf("ActivePresenter() = %v, want the installed one", got)
	}
	ResetPresenter()
	if _, ok := ActivePresenter().(SilentPresenter); !ok {
		t.Error("ResetPresenter left the previous one installed")
	}
}

// A presenter that was never installed must not come back as a nil interface: a
// nil Presenter would panic on the first call, which is the moment a user is
// watching.
func TestSetPresenterNilFallsBackToSilent(t *testing.T) {
	t.Cleanup(ResetPresenter)
	SetPresenter(nil)
	if _, ok := ActivePresenter().(SilentPresenter); !ok {
		t.Error("SetPresenter(nil) left a nil presenter installed")
	}
}

type countingPresenter struct {
	SilentPresenter
	sections, banners, kvs int
}

func (c *countingPresenter) Banner(string, string)  { c.banners++ }
func (c *countingPresenter) Section(string)         { c.sections++ }
func (c *countingPresenter) KV(int, string, string) { c.kvs++ }
