package ast

import "github.com/Aneesh495/fathom/internal/span"

// ClassKind distinguishes the four template-bearing declarations.
type ClassKind int

const (
	KindClass ClassKind = iota
	KindTrait
	KindObject
	KindEnum
)

func (k ClassKind) String() string {
	switch k {
	case KindTrait:
		return "trait"
	case KindObject:
		return "object"
	case KindEnum:
		return "enum"
	default:
		return "class"
	}
}

// ClassDecl is a class, trait, object, or enum.
type ClassDecl struct {
	S         span.Span
	Mods      []Mod
	Anns      []Annotation
	Kind      ClassKind
	Name      string
	TParams   []*TypeParam
	Params    []*Param
	HasParams bool
	Parents   []*Parent
	Body      *Template
	Cases     []*EnumCase
}

func (n *ClassDecl) Span() span.Span { return n.S }
func (n *ClassDecl) node()           {}
func (n *ClassDecl) stmt()           {}
func (n *ClassDecl) decl()           {}

// EnumCase is one case of an enum. HasParams false means a case object (`case Red`).
type EnumCase struct {
	S         span.Span
	Name      string
	Params    []*Param
	HasParams bool
	Body      *Template
}

func (n *EnumCase) Span() span.Span { return n.S }
func (n *EnumCase) node()           {}

// DefDecl is a method or function.
type DefDecl struct {
	S       span.Span
	Mods    []Mod
	Anns    []Annotation
	Name    string
	TParams []*TypeParam
	Lists   []*ParamList
	Result  TypeNode
	Body    Expr
}

func (n *DefDecl) Span() span.Span { return n.S }
func (n *DefDecl) node()           {}
func (n *DefDecl) stmt()           {}
func (n *DefDecl) decl()           {}

// ValDecl is `val` or `var`. Exactly one of Name or Pat is the binder.
// Name is set for the common case. Pat is set for `val (a, b) = expr`.
type ValDecl struct {
	S       span.Span
	Mods    []Mod
	Name    string
	Pat     Pattern
	Type    TypeNode
	Body    Expr
	Mutable bool
}

func (n *ValDecl) Span() span.Span { return n.S }
func (n *ValDecl) node()           {}
func (n *ValDecl) stmt()           {}
func (n *ValDecl) decl()           {}

// TypeDecl is a type alias or an abstract type member.
// Body set means an alias. Lo and Hi are bounds of an abstract type.
// Opaque types are aliases whose equality is hidden outside the companion.
type TypeDecl struct {
	S       span.Span
	Mods    []Mod
	Name    string
	TParams []*TypeParam
	Lo      TypeNode
	Hi      TypeNode
	Body    TypeNode
}

func (n *TypeDecl) Span() span.Span { return n.S }
func (n *TypeDecl) node()           {}
func (n *TypeDecl) stmt()           {}
func (n *TypeDecl) decl()           {}

// GivenDecl is a given instance. Name may be empty.
// Body and BodyTemplate are alternatives: `= expr` versus `with template`.
type GivenDecl struct {
	S            span.Span
	Mods         []Mod
	Name         string
	TParams      []*TypeParam
	Lists        []*ParamList
	Type         TypeNode
	Body         Expr
	BodyTemplate *Template
}

func (n *GivenDecl) Span() span.Span { return n.S }
func (n *GivenDecl) node()           {}
func (n *GivenDecl) stmt()           {}
func (n *GivenDecl) decl()           {}

// ExtDecl is an extension clause. Methods live in Body.
type ExtDecl struct {
	S       span.Span
	TParams []*TypeParam
	Param   *Param
	Body    *Template
}

func (n *ExtDecl) Span() span.Span { return n.S }
func (n *ExtDecl) node()           {}
func (n *ExtDecl) stmt()           {}
func (n *ExtDecl) decl()           {}
