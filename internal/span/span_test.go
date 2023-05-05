package span

import "testing"

func TestLineIndex(t *testing.T) {
	src := "ab\nc\r\n\nπx"
	f := NewFile("t.scala", src)
	if f.LineCount() != 4 {
		t.Fatalf("lines: got %d", f.LineCount())
	}
	cases := []struct {
		off      int
		line     int
		col      int
		lineText string
	}{
		{0, 1, 1, "ab"},
		{1, 1, 2, "ab"},
		{2, 1, 3, "ab"},
		{3, 2, 1, "c"},
		{6, 3, 1, ""},
		{7, 4, 1, "πx"},
	}
	for _, tc := range cases {
		p := f.Pos(tc.off)
		if p.Line != tc.line || p.Col != tc.col {
			t.Fatalf("offset %d: got %d:%d want %d:%d", tc.off, p.Line, p.Col, tc.line, tc.col)
		}
	}
	if got := f.LineText(4); got != "πx" {
		t.Fatalf("line 4: %q", got)
	}
	// π is two bytes. The column after it is 2, not 3.
	pi := f.Pos(7)
	if pi.Col != 1 {
		t.Fatalf("pi col %d", pi.Col)
	}
	after := f.Pos(7 + len("π"))
	if after.Line != 4 || after.Col != 2 {
		t.Fatalf("after pi: %d:%d", after.Line, after.Col)
	}
}

func TestSpanTextAndMerge(t *testing.T) {
	f := NewFile("a.scala", "hello")
	s := Span{File: f, Start: 1, End: 4}
	if s.Text() != "ell" {
		t.Fatalf("text %q", s.Text())
	}
	if s.Position().String() != "a.scala:1:2" {
		t.Fatalf("pos %s", s.Position())
	}
	m := Merge(Span{File: f, Start: 0, End: 1}, Span{File: f, Start: 3, End: 5})
	if m.Text() != "hello" {
		t.Fatalf("merge %q", m.Text())
	}
	if !(Span{}).IsZero() {
		t.Fatal("zero span")
	}
	if got := Merge(Span{}, s); got.Text() != "ell" {
		t.Fatalf("merge identity %q", got.Text())
	}
}

func TestClamp(t *testing.T) {
	f := NewFile("a.scala", "hi")
	p := f.Pos(100)
	if p.Offset != 2 || p.Line != 1 {
		t.Fatalf("clamp %+v", p)
	}
	if f.LineText(0) != "" || f.LineStart(-3) != 0 {
		t.Fatal("out of range line")
	}
	empty := NewFile("e.scala", "")
	if empty.LineCount() != 1 || empty.Pos(0).Col != 1 {
		t.Fatalf("empty %+v", empty.Pos(0))
	}
}
