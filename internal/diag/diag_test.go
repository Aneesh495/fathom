package diag

import (
	"strings"
	"testing"

	"github.com/Aneesh495/fathom/internal/span"
)

func TestRenderCaret(t *testing.T) {
	f := span.NewFile("a.scala", "val x: Int = \"hi\"\n")
	//             columns: 123456789012345678
	l := NewList(0)
	l.Errorf(span.Span{File: f, Start: 13, End: 17}, "type mismatch")
	got := l.Render()
	if !strings.Contains(got, "a.scala:1:14: error: type mismatch") {
		t.Fatalf("header:\n%s", got)
	}
	if !strings.Contains(got, "^^^^") {
		t.Fatalf("caret:\n%s", got)
	}
}

func TestLimitAndSort(t *testing.T) {
	f := span.NewFile("b.scala", "abc")
	l := NewList(1)
	l.Errorf(span.Span{File: f, Start: 2, End: 3}, "second")
	l.Errorf(span.Span{File: f, Start: 0, End: 1}, "first")
	l.Warnf(span.Span{File: f, Start: 0, End: 1}, "warn")
	if l.ErrorCount() != 1 {
		t.Fatalf("count %d", l.ErrorCount())
	}
	items := l.Sorted()
	if len(items) != 2 {
		t.Fatalf("items %d", len(items))
	}
	if items[0].Msg != "warn" && items[0].Msg != "second" {
		// The single error is at offset 2. The warning is at offset 0, so it sorts first.
		t.Fatalf("order %+v", items)
	}
	if items[0].Severity != Warning || items[0].Span.Start != 0 {
		t.Fatalf("first %+v", items[0])
	}
	if !l.ByMessage("second") || l.ByMessage("first") {
		t.Fatal("message search")
	}
}

func TestNoteAttached(t *testing.T) {
	f := span.NewFile("c.scala", "foo\nbar\n")
	l := NewList(0)
	l.Add(Diagnostic{
		Severity: Error,
		Span:     span.Span{File: f, Start: 0, End: 3},
		Msg:      "conflict",
		Notes: []Related{{
			Span: span.Span{File: f, Start: 4, End: 7},
			Msg:  "previous definition",
		}},
	})
	got := l.Render()
	if !strings.Contains(got, "note: previous definition") {
		t.Fatalf("note missing:\n%s", got)
	}
	if !strings.Contains(got, "2 | bar") {
		t.Fatalf("second line:\n%s", got)
	}
}
