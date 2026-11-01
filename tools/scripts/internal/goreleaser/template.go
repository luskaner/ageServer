package goreleaser

import (
	"bytes"
	"text/template"

	"github.com/luskaner/ageServer/common/uuid"
)

type Renders[D any] interface {
	Render(data D) string
}
type LiteralString[D any] string

func (t LiteralString[D]) Render(_ D) string {
	return string(t)
}

type Template[D any] struct {
	tmpl *template.Template
	text string
}

func NewTemplate[D any](text string) *Template[D] {
	tmpl := template.New(uuid.New().String())
	return &Template[D]{tmpl: tmpl, text: text}
}

func (t *Template[D]) Render(data D) string {
	tmpl, err := t.tmpl.Parse(t.text)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		// A failed execution may have written partial output; returning it
		// would silently corrupt the generated configuration.
		return ""
	}
	return buf.String()
}
