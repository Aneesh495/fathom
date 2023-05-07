package scanner

import (
	"strings"

	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

// lexInterp reads s"..." or raw"..." or the triple-quoted forms.
// The current rune is the opening quote. start is the offset of the
// interpolator name. The returned slice begins with InterpStart and ends
// with InterpEnd. Each hole is wrapped in parentheses so the parser can
// read it as one expression.
func (s *Scanner) lexInterp(name string, start int, raw bool, nl bool) []token.Token {
	quoteAt := s.off
	triple := s.peek() == '"' && s.look(2) == '"'
	s.next() // first quote
	if triple {
		s.next()
		s.next()
	}
	var out []token.Token
	out = append(out, token.Token{
		Kind:    token.InterpStart,
		Lit:     name,
		Span:    s.span(start, s.off),
		Newline: nl,
	})
	_ = quoteAt

	for {
		chunkStart := s.off
		chunk, how := s.readChunk(raw, triple)
		out = append(out, token.Token{
			Kind: token.StrPart,
			Lit:  chunk,
			Span: s.span(chunkStart, s.off),
		})
		switch how {
		case chunkEnd:
			out = append(out, token.Token{Kind: token.InterpEnd, Span: s.span(s.off, s.off)})
			// The closing quotes were consumed by readChunk. InterpEnd's span
			// is zero-width at the end, which is enough for an EOF-adjacent caret.
			if len(out) > 0 {
				end := s.off
				out[len(out)-1].Span = s.span(end, end)
			}
			return out
		case chunkIdent:
			idStart := s.off
			var b strings.Builder
			for isIdentCont(s.ch) {
				b.WriteRune(s.ch)
				s.next()
			}
			id := b.String()
			idSpan := s.span(idStart, s.off)
			out = append(out,
				token.Token{Kind: token.LParen, Span: idSpan},
				token.Token{Kind: token.Ident, Lit: id, Span: idSpan},
				token.Token{Kind: token.RParen, Span: span.Span{File: s.file, Start: s.off, End: s.off}},
			)
		case chunkBrace:
			// `${` has been consumed, including the brace.
			open := s.off
			hole := s.readHole()
			out = append(out, token.Token{Kind: token.LParen, Span: s.span(open, open)})
			out = append(out, hole...)
			out = append(out, token.Token{Kind: token.RParen, Span: s.span(s.off, s.off)})
		case chunkError:
			out = append(out, token.Token{Kind: token.InterpEnd, Span: s.span(s.off, s.off)})
			return out
		}
	}
}

type chunkHow int

const (
	chunkEnd chunkHow = iota
	chunkIdent
	chunkBrace
	chunkError
)

// readChunk consumes one literal piece. It leaves the scanner on the first
// rune of a hole, or just past the closing quotes when the string ends.
func (s *Scanner) readChunk(raw bool, triple bool) (string, chunkHow) {
	var b strings.Builder
	for s.ch != -1 {
		if !triple && s.ch == '\n' {
			s.error(s.span(s.off, s.off+1), "unterminated string interpolation")
			return b.String(), chunkError
		}
		if s.ch == '"' {
			if triple {
				if s.peek() == '"' && s.look(2) == '"' {
					s.next()
					s.next()
					s.next()
					return b.String(), chunkEnd
				}
				b.WriteRune(s.ch)
				s.next()
				continue
			}
			s.next()
			return b.String(), chunkEnd
		}
		if s.ch == '$' {
			s.next()
			if s.ch == '$' {
				b.WriteByte('$')
				s.next()
				continue
			}
			if isIdentStart(s.ch) {
				return b.String(), chunkIdent
			}
			if s.ch == '{' {
				s.next()
				return b.String(), chunkBrace
			}
			if s.ch == -1 {
				return b.String(), chunkError
			}
			s.error(s.span(s.off, s.off+1), "interpolation hole must be $ident or ${expr}")
			continue
		}
		if s.ch == '\\' && !raw && !triple {
			s.next()
			r, ok := s.readEscape(true)
			if ok {
				b.WriteRune(r)
			}
			continue
		}
		b.WriteRune(s.ch)
		s.next()
	}
	s.error(s.span(s.off, s.off), "unterminated string interpolation")
	return b.String(), chunkError
}

// readHole reads tokens until the brace that opened the hole closes.
// Nested braces, strings, and interpolations are tokenized normally.
func (s *Scanner) readHole() []token.Token {
	var out []token.Token
	depth := 1
	for s.ch != -1 && depth > 0 {
		t := s.nextToken()
		if t.Kind == token.EOF {
			s.error(t.Span, "unterminated interpolation hole")
			break
		}
		if t.Kind == token.LBrace {
			depth++
		} else if t.Kind == token.RBrace {
			depth--
			if depth == 0 {
				break
			}
		}
		out = append(out, t)
	}
	return out
}
