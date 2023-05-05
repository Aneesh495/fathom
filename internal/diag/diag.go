// Package diag collects and renders compiler diagnostics.
// Rendering is deterministic: diagnostics are ordered by file, offset, and message.
package diag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Aneesh495/fathom/internal/span"
)

// Severity is how seriously the compiler takes a note.
type Severity int

const (
	Note Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Note:
		return "note"
	case Warning:
		return "warning"
	default:
		return "error"
	}
}

// Diagnostic is one message anchored at a span, with optional related notes.
type Diagnostic struct {
	Severity Severity
	Span     span.Span
	Msg      string
	Notes    []Related
}

// Related is a secondary span that explains a diagnostic.
type Related struct {
	Span span.Span
	Msg  string
}

// List is an ordered bag of diagnostics. The zero value is ready to use.
type List struct {
	Items []Diagnostic
	Limit int
}

// NewList returns a list that stops recording after limit errors.
// Non-positive limits mean "keep everything".
func NewList(limit int) *List {
	return &List{Limit: limit}
}

// Errorf records an error.
func (l *List) Errorf(sp span.Span, format string, args ...any) {
	l.add(Error, sp, fmt.Sprintf(format, args...), nil)
}

// Warnf records a warning.
func (l *List) Warnf(sp span.Span, format string, args ...any) {
	l.add(Warning, sp, fmt.Sprintf(format, args...), nil)
}

// Notef records a standalone note.
func (l *List) Notef(sp span.Span, format string, args ...any) {
	l.add(Note, sp, fmt.Sprintf(format, args...), nil)
}

// Add records a fully built diagnostic.
func (l *List) Add(d Diagnostic) {
	l.add(d.Severity, d.Span, d.Msg, d.Notes)
}

func (l *List) add(sev Severity, sp span.Span, msg string, notes []Related) {
	if l == nil {
		return
	}
	if l.Limit > 0 && sev == Error && l.ErrorCount() >= l.Limit {
		return
	}
	l.Items = append(l.Items, Diagnostic{Severity: sev, Span: sp, Msg: msg, Notes: notes})
}

// ErrorCount counts severity Error.
func (l *List) ErrorCount() int {
	if l == nil {
		return 0
	}
	n := 0
	for _, d := range l.Items {
		if d.Severity == Error {
			n++
		}
	}
	return n
}

// HasErrors reports whether any error was recorded.
func (l *List) HasErrors() bool { return l.ErrorCount() > 0 }

// Sorted returns a copy ordered by file name, start offset, severity, and message.
func (l *List) Sorted() []Diagnostic {
	if l == nil {
		return nil
	}
	out := append([]Diagnostic(nil), l.Items...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		af, bf := "", ""
		ao, bo := 0, 0
		if a.Span.File != nil {
			af = a.Span.File.Name
			ao = a.Span.Start
		}
		if b.Span.File != nil {
			bf = b.Span.File.Name
			bo = b.Span.Start
		}
		if af != bf {
			return af < bf
		}
		if ao != bo {
			return ao < bo
		}
		if a.Severity != b.Severity {
			return a.Severity > b.Severity
		}
		return a.Msg < b.Msg
	})
	return out
}

// Render writes every diagnostic with a source line and a caret.
// The layout is stable so tests can match it as text.
func (l *List) Render() string {
	var b strings.Builder
	items := l.Sorted()
	for i, d := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		writeOne(&b, d.Severity, d.Span, d.Msg)
		for _, n := range d.Notes {
			b.WriteByte('\n')
			writeOne(&b, Note, n.Span, n.Msg)
		}
	}
	return b.String()
}

func writeOne(b *strings.Builder, sev Severity, sp span.Span, msg string) {
	pos := sp.Position()
	fmt.Fprintf(b, "%s: %s: %s", pos.String(), sev.String(), msg)
	if sp.File == nil {
		return
	}
	line := sp.File.LineText(pos.Line)
	if line == "" && sp.Start >= len(sp.File.Src) {
		return
	}
	b.WriteByte('\n')
	fmt.Fprintf(b, "%4d | %s\n", pos.Line, line)
	b.WriteString("     | ")
	// Caret column is 1-based. A tab in source counts as one column here,
	// which matches the column counter in the scanner.
	for i := 1; i < pos.Col; i++ {
		b.WriteByte(' ')
	}
	width := caretWidth(sp)
	if width < 1 {
		width = 1
	}
	for i := 0; i < width; i++ {
		b.WriteByte('^')
	}
}

func caretWidth(sp span.Span) int {
	if sp.File == nil || sp.End <= sp.Start {
		return 1
	}
	start := sp.Position()
	end := sp.EndPosition()
	if start.Line != end.Line {
		// Multi-line spans underline only the first line's remainder.
		text := sp.File.LineText(start.Line)
		w := utf8Len(text) - (start.Col - 1)
		if w < 1 {
			return 1
		}
		return w
	}
	w := end.Col - start.Col
	if w < 1 {
		return 1
	}
	return w
}

func utf8Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// ByMessage reports whether any diagnostic contains substr.
func (l *List) ByMessage(substr string) bool {
	if l == nil {
		return false
	}
	for _, d := range l.Items {
		if strings.Contains(d.Msg, substr) {
			return true
		}
		for _, n := range d.Notes {
			if strings.Contains(n.Msg, substr) {
				return true
			}
		}
	}
	return false
}
