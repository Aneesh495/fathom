package ast

import "github.com/Aneesh495/fathom/internal/span"

// IdentType is a bare type name.
type IdentType struct {
	S    span.Span
	Name string
}

func (n *IdentType) Span() span.Span { return n.S }
func (n *IdentType) node()           {}
func (n *IdentType) typeNode()       {}

// SelectType is `prefix.Name` in type position.
// The checker decides whether the prefix is a package, a type, or a term path.
type SelectType struct {
	S    span.Span
	X    TypeNode
	Name string
}

func (n *SelectType) Span() span.Span { return n.S }
func (n *SelectType) node()           {}
func (n *SelectType) typeNode()       {}

// ProjectType is the type projection `T#A`.
type ProjectType struct {
	S    span.Span
	X    TypeNode
	Name string
}

func (n *ProjectType) Span() span.Span { return n.S }
func (n *ProjectType) node()           {}
func (n *ProjectType) typeNode()       {}

// ApplyType is `T[U, V]`.
type ApplyType struct {
	S    span.Span
	Fun  TypeNode
	Args []TypeNode
}

func (n *ApplyType) Span() span.Span { return n.S }
func (n *ApplyType) node()           {}
func (n *ApplyType) typeNode()       {}

// FunType is a function type. One parameter is not wrapped in a tuple.
type FunType struct {
	S      span.Span
	Params []TypeNode
	Result TypeNode
}

func (n *FunType) Span() span.Span { return n.S }
func (n *FunType) node()           {}
func (n *FunType) typeNode()       {}

// TupleType is `(A, B, ...)`.
type TupleType struct {
	S     span.Span
	Elems []TypeNode
}

func (n *TupleType) Span() span.Span { return n.S }
func (n *TupleType) node()           {}
func (n *TupleType) typeNode()       {}

// InfixType is `A | B` or `A & B`.
type InfixType struct {
	S           span.Span
	Op          string
	Left, Right TypeNode
}

func (n *InfixType) Span() span.Span { return n.S }
func (n *InfixType) node()           {}
func (n *InfixType) typeNode()       {}

// RefineType is `T { members }`.
type RefineType struct {
	S     span.Span
	X     TypeNode
	Stats []Stmt
}

func (n *RefineType) Span() span.Span { return n.S }
func (n *RefineType) node()           {}
func (n *RefineType) typeNode()       {}

// PathType is a path-dependent type whose prefix is clearly a term, such as `this.Elem`.
// A dotted name that might be either a term or a type is a SelectType until the checker decides.
type PathType struct {
	S      span.Span
	Prefix Expr
	Name   string
}

func (n *PathType) Span() span.Span { return n.S }
func (n *PathType) node()           {}
func (n *PathType) typeNode()       {}

// SingletonType is `path.type`.
type SingletonType struct {
	S    span.Span
	Path Expr
}

func (n *SingletonType) Span() span.Span { return n.S }
func (n *SingletonType) node()           {}
func (n *SingletonType) typeNode()       {}

// ByNameType is `=> T`.
type ByNameType struct {
	S span.Span
	X TypeNode
}

func (n *ByNameType) Span() span.Span { return n.S }
func (n *ByNameType) node()           {}
func (n *ByNameType) typeNode()       {}

// WildType is `?` or `_`, with optional bounds.
type WildType struct {
	S  span.Span
	Lo TypeNode
	Hi TypeNode
}

func (n *WildType) Span() span.Span { return n.S }
func (n *WildType) node()           {}
func (n *WildType) typeNode()       {}

// LitType is a literal singleton type such as `1` or `"a"`.
type LitType struct {
	S   span.Span
	Lit *LiteralExpr
}

func (n *LitType) Span() span.Span { return n.S }
func (n *LitType) node()           {}
func (n *LitType) typeNode()       {}
