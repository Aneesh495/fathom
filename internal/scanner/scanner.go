// Package scanner tokenizes Fathom source.
//
// Newline handling follows the Scala brace dialect, with one deliberate
// tightening: a newline before '{' ends a statement. Block arguments are
// written on the same line (`f { 1 }`) or with parentheses. A newline before
// '(' or '[' does not end a statement, so curried calls and type arguments
// can break there.
//
// Unicode escapes are processed inside character and string literals only.
// They are not a source-wide pre-pass. That keeps the column of every other
// token identical to the file the author sees.
package scanner

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Aneesh495/fathom/internal/diag"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

// Scan tokenizes file. The result always ends with EOF.
// Diagnostics are recorded for lexical problems; a partial token stream is
// still returned so the parser can report additional structure errors.
func Scan(file *span.File) ([]token.Token, *diag.List) {
	s := &Scanner{
		file: file,
		src:  "",
		line: 1,
	}
	if file != nil {
		s.src = file.Src
	}
	s.next()
	var raw []token.Token
	for {
		tok := s.nextToken()
		raw = append(raw, tok)
		if tok.Kind == token.EOF {
			break
		}
		if len(raw) > len(s.src)+8 && len(raw) > 8 {
			// Pathological input should not loop forever. Each token consumes
			// at least one byte except synthetic interpolation parens, which
			// are bounded by the number of holes.
			if len(raw) > len(s.src)*4+64 {
				s.error(tok.Span, "lexer failed to make progress")
				break
			}
		}
	}
	return insertSemis(raw), s.diags
}

// Scanner is exposed for tests that want a single-file lexer state.
// Prefer Scan.
type Scanner struct {
	file *span.File
	src  string
	pos  int  // byte index of the next unread byte
	off  int  // byte index of ch
	ch   rune // current rune, -1 at EOF
	line int
	col  int

	paren int
	brack int

	pending []token.Token
	diags   *diag.List
}

func (s *Scanner) next() {
	if s.pos >= len(s.src) {
		s.off = len(s.src)
		s.ch = -1
		return
	}
	if s.ch == '\n' {
		s.line++
		s.col = 0
	}
	s.off = s.pos
	r, w := utf8.DecodeRuneInString(s.src[s.pos:])
	if r == utf8.RuneError && w == 1 {
		s.pos += w
		s.ch = r
		s.col++
		return
	}
	s.pos += w
	s.ch = r
	s.col++
}

func (s *Scanner) peek() rune {
	if s.pos >= len(s.src) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(s.src[s.pos:])
	return r
}

func (s *Scanner) span(start, end int) span.Span {
	return span.Span{File: s.file, Start: start, End: end}
}

func (s *Scanner) error(sp span.Span, msg string) {
	if s.diags == nil {
		s.diags = diag.NewList(0)
	}
	s.diags.Errorf(sp, "%s", msg)
}

func (s *Scanner) nextToken() token.Token {
	if len(s.pending) > 0 {
		t := s.pending[0]
		s.pending = s.pending[1:]
		return t
	}
	return s.lex()
}

func (s *Scanner) lex() token.Token {
	nl := s.skip()
	if s.ch == -1 {
		return token.Token{Kind: token.EOF, Span: s.span(s.off, s.off), Newline: nl}
	}
	start := s.off

	switch s.ch {
	case '(':
		s.next()
		s.paren++
		return s.simple(token.LParen, start, nl)
	case ')':
		s.next()
		if s.paren > 0 {
			s.paren--
		}
		return s.simple(token.RParen, start, nl)
	case '[':
		s.next()
		s.brack++
		return s.simple(token.LBrack, start, nl)
	case ']':
		s.next()
		if s.brack > 0 {
			s.brack--
		}
		return s.simple(token.RBrack, start, nl)
	case '{':
		s.next()
		return s.simple(token.LBrace, start, nl)
	case '}':
		s.next()
		return s.simple(token.RBrace, start, nl)
	case ',':
		s.next()
		return s.simple(token.Comma, start, nl)
	case ';':
		s.next()
		return s.simple(token.Semi, start, nl)
	case '.':
		s.next()
		return s.simple(token.Dot, start, nl)
	case '\'':
		return s.lexChar(start, nl)
	case '"':
		return s.lexPlainString(start, nl)
	case '`':
		return s.lexBacktick(start, nl)
	}

	if s.ch == '/' && (s.peek() == '/' || s.peek() == '*') {
		// skip() should have consumed comments. This is a safety net.
		s.scanComment()
		return s.lex()
	}

	if isDigit(s.ch) {
		return s.lexNumber(start, nl)
	}
	if isIdentStart(s.ch) {
		return s.lexIdent(start, nl)
	}
	if isOpChar(s.ch) {
		return s.lexOp(start, nl)
	}

	r := s.ch
	s.next()
	sp := s.span(start, s.off)
	s.error(sp, fmt.Sprintf("illegal character %q", string(r)))
	return s.lex()
}

func (s *Scanner) simple(kind token.Kind, start int, nl bool) token.Token {
	return token.Token{Kind: kind, Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) skip() bool {
	newline := false
	for {
		switch s.ch {
		case ' ', '\t', '\f':
			s.next()
		case '\r':
			s.next()
			if s.paren == 0 && s.brack == 0 {
				newline = true
			}
			if s.ch == '\n' {
				s.next()
			}
		case '\n':
			if s.paren == 0 && s.brack == 0 {
				newline = true
			}
			s.next()
		case '/':
			if s.peek() == '/' || s.peek() == '*' {
				s.scanComment()
				continue
			}
			return newline
		case '\uFEFF':
			s.next()
		default:
			return newline
		}
	}
}

func (s *Scanner) scanComment() {
	start := s.off
	if s.ch == '/' && s.peek() == '/' {
		s.next()
		s.next()
		for s.ch != -1 && s.ch != '\n' {
			s.next()
		}
		return
	}
	if s.ch == '/' && s.peek() == '*' {
		s.next()
		s.next()
		depth := 1
		for s.ch != -1 && depth > 0 {
			if s.ch == '/' && s.peek() == '*' {
				s.next()
				s.next()
				depth++
				continue
			}
			if s.ch == '*' && s.peek() == '/' {
				s.next()
				s.next()
				depth--
				continue
			}
			s.next()
		}
		if depth != 0 {
			s.error(s.span(start, s.off), "unterminated comment")
		}
	}
}

func (s *Scanner) lexIdent(start int, nl bool) token.Token {
	for isIdentCont(s.ch) {
		s.next()
	}
	lit := s.src[start:s.off]
	if (lit == "s" || lit == "raw") && s.ch == '"' {
		toks := s.lexInterp(lit, start, lit == "raw", nl)
		if len(toks) == 0 {
			return token.Token{Kind: token.EOF, Span: s.span(s.off, s.off)}
		}
		s.pending = append(toks[1:], s.pending...)
		return toks[0]
	}
	if kind, ok := token.Keyword(lit); ok {
		return token.Token{Kind: kind, Lit: lit, Span: s.span(start, s.off), Newline: nl}
	}
	return token.Token{Kind: token.Ident, Lit: lit, Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) lexBacktick(start int, nl bool) token.Token {
	s.next() // opening `
	var b strings.Builder
	for s.ch != -1 && s.ch != '`' && s.ch != '\n' {
		b.WriteRune(s.ch)
		s.next()
	}
	if s.ch != '`' {
		sp := s.span(start, s.off)
		s.error(sp, "unterminated backtick identifier")
		return token.Token{Kind: token.Ident, Lit: b.String(), Span: sp, Newline: nl}
	}
	s.next()
	if b.Len() == 0 {
		sp := s.span(start, s.off)
		s.error(sp, "empty backtick identifier")
	}
	return token.Token{Kind: token.Ident, Lit: b.String(), Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) lexNumber(start int, nl bool) token.Token {
	baseHint := 10
	if s.ch == '0' && (s.peek() == 'x' || s.peek() == 'X' || s.peek() == 'b' || s.peek() == 'B') {
		if s.peek() == 'x' || s.peek() == 'X' {
			baseHint = 16
		} else {
			baseHint = 2
		}
		s.next()
		s.next()
		n := 0
		for isRadixDigit(s.ch, baseHint) || s.ch == '_' {
			if s.ch != '_' {
				n++
			}
			s.next()
		}
		lit := s.src[start:s.off]
		sp := s.span(start, s.off)
		if n == 0 || strings.HasSuffix(lit, "_") || strings.Contains(lit, "__") {
			s.error(sp, "malformed integer literal")
		}
		return token.Token{Kind: token.Int, Lit: lit, Span: sp, Newline: nl}
	}

	kind := token.Int
	s.scanDigits()
	if s.ch == '.' && isDigit(s.peek()) {
		kind = token.Float
		s.next()
		s.scanDigits()
	}
	if s.ch == 'e' || s.ch == 'E' {
		kind = token.Float
		s.next()
		if s.ch == '+' || s.ch == '-' {
			s.next()
		}
		if !isDigit(s.ch) {
			sp := s.span(start, s.off)
			s.error(sp, "malformed floating point literal")
		}
		s.scanDigits()
	}
	lit := s.src[start:s.off]
	sp := s.span(start, s.off)
	if strings.HasSuffix(lit, "_") || strings.Contains(lit, "__") {
		s.error(sp, "malformed numeric literal")
	}
	return token.Token{Kind: kind, Lit: lit, Span: sp, Newline: nl}
}

func (s *Scanner) scanDigits() {
	for isDigit(s.ch) || s.ch == '_' {
		s.next()
	}
}

func (s *Scanner) lexOp(start int, nl bool) token.Token {
	for isOpChar(s.ch) {
		s.next()
	}
	lit := s.src[start:s.off]
	kind := token.Op
	switch lit {
	case "=>":
		kind = token.Arrow
	case "<-":
		kind = token.LArrow
	case "<:":
		kind = token.Subtype
	case ">:":
		kind = token.Supertype
	case ":":
		kind = token.Colon
	case "=":
		kind = token.Assign
	case "@":
		kind = token.At
	case "?":
		kind = token.Question
	case "#":
		kind = token.Hash
	}
	return token.Token{Kind: kind, Lit: lit, Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) lexChar(start int, nl bool) token.Token {
	s.next() // '
	if s.ch == -1 || s.ch == '\n' {
		sp := s.span(start, s.off)
		s.error(sp, "unterminated character literal")
		return token.Token{Kind: token.Char, Span: sp, Newline: nl}
	}
	r, ok := s.readEscape(false)
	if !ok {
		sp := s.span(start, s.off)
		return token.Token{Kind: token.Char, Span: sp, Newline: nl}
	}
	if s.ch != '\'' {
		// Consume until quote or end so one bad literal is one error.
		for s.ch != -1 && s.ch != '\'' && s.ch != '\n' {
			s.next()
		}
		sp := s.span(start, s.off)
		if s.ch == '\'' {
			s.next()
			sp = s.span(start, s.off)
		}
		s.error(sp, "character literal must contain exactly one character")
		return token.Token{Kind: token.Char, Span: sp, Newline: nl}
	}
	s.next()
	return token.Token{Kind: token.Char, Lit: string(r), Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) lexPlainString(start int, nl bool) token.Token {
	if s.peek() == '"' && s.look(2) == '"' {
		return s.lexTriple(start, nl, false)
	}
	s.next() // opening "
	var b strings.Builder
	for s.ch != -1 && s.ch != '"' && s.ch != '\n' {
		if s.ch == '\\' {
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
	if s.ch != '"' {
		sp := s.span(start, s.off)
		s.error(sp, "unterminated string")
		return token.Token{Kind: token.String, Lit: b.String(), Span: sp, Newline: nl}
	}
	s.next()
	return token.Token{Kind: token.String, Lit: b.String(), Span: s.span(start, s.off), Newline: nl}
}

func (s *Scanner) lexTriple(start int, nl bool, raw bool) token.Token {
	// Current rune is the first of three quotes, or for a call from
	// lexPlainString the first quote is still current.
	s.next()
	s.next()
	s.next()
	var b strings.Builder
	for s.ch != -1 {
		if s.ch == '"' && s.peek() == '"' && s.look(2) == '"' {
			s.next()
			s.next()
			s.next()
			return token.Token{Kind: token.String, Lit: b.String(), Span: s.span(start, s.off), Newline: nl}
		}
		if !raw && s.ch == '\\' {
			// Triple-quoted strings are raw. `raw` here means the interpolator
			// raw"""...""", which is also raw. Ordinary """ is raw too.
		}
		b.WriteRune(s.ch)
		s.next()
	}
	sp := s.span(start, s.off)
	s.error(sp, "unterminated triple-quoted string")
	return token.Token{Kind: token.String, Lit: b.String(), Span: sp, Newline: nl}
}

// look returns the rune n steps after the current one.
// look(1) is the immediate successor, the same rune peek reports.
func (s *Scanner) look(n int) rune {
	if n < 1 {
		return s.ch
	}
	p := s.pos
	var r rune = -1
	for i := 0; i < n; i++ {
		if p >= len(s.src) {
			return -1
		}
		rr, w := utf8.DecodeRuneInString(s.src[p:])
		p += w
		r = rr
	}
	return r
}

func (s *Scanner) readEscape(alreadyConsumedSlash bool) (rune, bool) {
	if !alreadyConsumedSlash {
		if s.ch != '\\' {
			r := s.ch
			s.next()
			return r, true
		}
		s.next()
	}
	if s.ch == -1 {
		s.error(s.span(s.off, s.off), "unterminated escape")
		return 0, false
	}
	switch s.ch {
	case 'n':
		s.next()
		return '\n', true
	case 't':
		s.next()
		return '\t', true
	case 'r':
		s.next()
		return '\r', true
	case 'b':
		s.next()
		return '\b', true
	case 'f':
		s.next()
		return '\f', true
	case '\\', '\'', '"', '$':
		r := s.ch
		s.next()
		return r, true
	case 'u':
		return s.readUnicode()
	default:
		sp := s.span(s.off, s.off+1)
		s.error(sp, fmt.Sprintf("unknown string escape \\%c", s.ch))
		s.next()
		return 0, false
	}
}

func (s *Scanner) readUnicode() (rune, bool) {
	start := s.off
	s.next() // u
	hex := ""
	if s.ch == '{' {
		s.next()
		var b strings.Builder
		for isHex(s.ch) {
			b.WriteRune(s.ch)
			s.next()
		}
		if s.ch != '}' || b.Len() == 0 || b.Len() > 6 {
			s.error(s.span(start-1, s.off), "malformed unicode escape")
			if s.ch == '}' {
				s.next()
			}
			return 0, false
		}
		s.next()
		hex = b.String()
	} else {
		var b strings.Builder
		for i := 0; i < 4; i++ {
			if !isHex(s.ch) {
				s.error(s.span(start-1, s.off), "unicode escape requires four hex digits or {hex}")
				return 0, false
			}
			b.WriteRune(s.ch)
			s.next()
		}
		hex = b.String()
	}
	var v rune
	for _, c := range hex {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v += c - '0'
		case c >= 'a' && c <= 'f':
			v += c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v += c - 'A' + 10
		}
	}
	if v > unicode.MaxRune {
		s.error(s.span(start, s.off), "unicode escape is out of range")
		return 0, false
	}
	return v, true
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isHex(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isRadixDigit(r rune, base int) bool {
	switch base {
	case 2:
		return r == '0' || r == '1'
	case 16:
		return isHex(r)
	default:
		return isDigit(r)
	}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentCont(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isOpChar(r rune) bool {
	switch r {
	case '!', '#', '%', '&', '*', '+', '-', '/', ':', '<', '=', '>', '?', '@', '\\', '^', '|', '~':
		return true
	default:
		return false
	}
}
