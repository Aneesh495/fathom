package ast

import (
	"unicode"
	"unicode/utf8"

	"github.com/Aneesh495/fathom/internal/span"
)

// WildPat is `_`.
type WildPat struct {
	S span.Span
}

func (n *WildPat) Span() span.Span { return n.S }
func (n *WildPat) node()           {}
func (n *WildPat) pattern()        {}

// BindPat is a variable binding, or `name @ pat`.
type BindPat struct {
	S    span.Span
	Name string
	Pat  Pattern
}

func (n *BindPat) Span() span.Span { return n.S }
func (n *BindPat) node()           {}
func (n *BindPat) pattern()        {}

// LitPat is a constant pattern.
type LitPat struct {
	S   span.Span
	Lit *LiteralExpr
}

func (n *LitPat) Span() span.Span { return n.S }
func (n *LitPat) node()           {}
func (n *LitPat) pattern()        {}

// TypedPat is `pat: Type`.
type TypedPat struct {
	S    span.Span
	Pat  Pattern
	Type TypeNode
}

func (n *TypedPat) Span() span.Span { return n.S }
func (n *TypedPat) node()           {}
func (n *TypedPat) pattern()        {}

// CtorPat is an extractor or case-class pattern. Pats nil means `Nil` with no parameter list.
// HasArgs distinguishes `Red` from `Red()`.
type CtorPat struct {
	S       span.Span
	Ctor    Expr
	Pats    []Pattern
	HasArgs bool
}

func (n *CtorPat) Span() span.Span { return n.S }
func (n *CtorPat) node()           {}
func (n *CtorPat) pattern()        {}

// TuplePat is `(a, b)`.
type TuplePat struct {
	S    span.Span
	Pats []Pattern
}

func (n *TuplePat) Span() span.Span { return n.S }
func (n *TuplePat) node()           {}
func (n *TuplePat) pattern()        {}

// AltPat is `p | q`.
type AltPat struct {
	S    span.Span
	Pats []Pattern
}

func (n *AltPat) Span() span.Span { return n.S }
func (n *AltPat) node()           {}
func (n *AltPat) pattern()        {}

// IsVarName reports whether a pattern identifier is a fresh binder.
// Scala's rule: a lowercase start is a binder, anything else is a stable name.
// `_` is a wildcard, not a binder. Quoted identifiers are never binders.
func IsVarName(name string, quoted bool) bool {
	if quoted || name == "" || name == "_" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLower(r)
}
