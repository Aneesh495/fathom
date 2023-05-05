// Package span maps byte offsets back to lines and columns.
// Every diagnostic, token, and AST node points at a span in one of these files.
package span

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// File is an immutable source buffer plus a line index.
// Columns count Unicode code points, and both lines and columns are 1-based.
type File struct {
	Name string
	Src  string
	// lineStarts holds the byte offset of each line. Index 0 is line 1.
	lineStarts []int
}

// NewFile indexes newlines in src. The file retains src as given.
func NewFile(name, src string) *File {
	f := &File{Name: name, Src: src}
	f.lineStarts = []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			f.lineStarts = append(f.lineStarts, i+1)
		}
	}
	return f
}

// Size is the length of the buffer in bytes.
func (f *File) Size() int {
	if f == nil {
		return 0
	}
	return len(f.Src)
}

// LineCount reports how many lines the buffer occupies.
// An empty buffer still has one line, matching editor conventions.
func (f *File) LineCount() int {
	if f == nil || len(f.lineStarts) == 0 {
		return 1
	}
	return len(f.lineStarts)
}

// LineStart returns the byte offset where the 1-based line begins.
// Out of range lines clamp to the nearest real line.
func (f *File) LineStart(line int) int {
	if f == nil || len(f.lineStarts) == 0 {
		return 0
	}
	if line < 1 {
		return f.lineStarts[0]
	}
	if line > len(f.lineStarts) {
		return f.lineStarts[len(f.lineStarts)-1]
	}
	return f.lineStarts[line-1]
}

// Position is a human-facing coordinate inside a file.
type Position struct {
	File   string
	Line   int
	Col    int
	Offset int
}

func (p Position) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// Pos resolves a byte offset. Offsets past the end resolve to the position
// just after the last character, which is where EOF diagnostics land.
func (f *File) Pos(offset int) Position {
	if f == nil {
		return Position{Line: 1, Col: 1}
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(f.Src) {
		offset = len(f.Src)
	}
	line := f.lineOf(offset)
	start := f.lineStarts[line-1]
	col := 1
	for i := start; i < offset; {
		_, size := utf8.DecodeRuneInString(f.Src[i:])
		if size <= 0 {
			break
		}
		i += size
		col++
	}
	return Position{File: f.Name, Line: line, Col: col, Offset: offset}
}

func (f *File) lineOf(offset int) int {
	// Binary search the greatest line start that is <= offset.
	lo, hi := 0, len(f.lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if f.lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1
}

// LineText returns the text of a 1-based line without its trailing newline.
func (f *File) LineText(line int) string {
	if f == nil || line < 1 || line > len(f.lineStarts) {
		return ""
	}
	start := f.lineStarts[line-1]
	end := len(f.Src)
	if line < len(f.lineStarts) {
		end = f.lineStarts[line] - 1
	}
	if end < start {
		end = start
	}
	text := f.Src[start:end]
	text = strings.TrimSuffix(text, "\r")
	return text
}

// Span is a half-open byte range [Start, End) inside File.
type Span struct {
	File  *File
	Start int
	End   int
}

// IsZero reports whether the span has no file.
func (s Span) IsZero() bool { return s.File == nil }

// Position returns the start coordinate.
func (s Span) Position() Position {
	if s.File == nil {
		return Position{Line: 1, Col: 1}
	}
	return s.File.Pos(s.Start)
}

// EndPosition returns the coordinate of the first byte after the span.
func (s Span) EndPosition() Position {
	if s.File == nil {
		return Position{Line: 1, Col: 1}
	}
	return s.File.Pos(s.End)
}

// Text returns the covered source. Invalid ranges return an empty string.
func (s Span) Text() string {
	if s.File == nil {
		return ""
	}
	start, end := s.Start, s.End
	if start < 0 {
		start = 0
	}
	if end > len(s.File.Src) {
		end = len(s.File.Src)
	}
	if start > end {
		return ""
	}
	return s.File.Src[start:end]
}

// Merge extends a span so that it covers both inputs.
// The result uses a's file. A zero span is the identity.
func Merge(a, b Span) Span {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	start, end := a.Start, a.End
	if b.Start < start {
		start = b.Start
	}
	if b.End > end {
		end = b.End
	}
	return Span{File: a.File, Start: start, End: end}
}

// Point is a zero-width span at offset.
func Point(f *File, offset int) Span {
	if f == nil {
		return Span{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(f.Src) {
		offset = len(f.Src)
	}
	return Span{File: f, Start: offset, End: offset}
}
