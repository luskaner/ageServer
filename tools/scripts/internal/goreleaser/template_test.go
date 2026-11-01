package goreleaser

import (
	"testing"
)

type renderProbe struct{ A string }

// Regression: Render used to return the partial buffer written before an
// execution error, silently corrupting the generated configuration.
func TestTemplateRenderErrorReturnsEmpty(t *testing.T) {
	tpl := NewTemplate[renderProbe]("pre {{.A}} {{.Missing}}")
	got := tpl.Render(renderProbe{A: "va"})
	if got != "" {
		t.Fatalf("Render on execute error = %q, want empty string (partial output %q leaked)", got, "pre va ")
	}
}

func TestTemplateRenderParseErrorReturnsEmpty(t *testing.T) {
	tpl := NewTemplate[renderProbe]("{{.A")
	if got := tpl.Render(renderProbe{A: "x"}); got != "" {
		t.Fatalf("Render on parse error = %q, want empty", got)
	}
}

func TestTemplateRenderHappyPath(t *testing.T) {
	tpl := NewTemplate[renderProbe]("pre {{.A}} post")
	if got := tpl.Render(renderProbe{A: "mid"}); got != "pre mid post" {
		t.Fatalf("Render = %q", got)
	}
}

func TestLiteralStringRenderIgnoresData(t *testing.T) {
	if got := LiteralString[renderProbe]("static").Render(renderProbe{A: "x"}); got != "static" {
		t.Fatalf("LiteralString = %q", got)
	}
}
