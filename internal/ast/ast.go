// Package ast is the surface syntax of Fathom.
//
// Nodes are pointer types. A later pass may attach types by pointer identity,
// so nodes must not be copied by value once they have been linked. Positions
// are byte offsets into the originating file.
package ast

import "github.com/Aneesh495/fathom/internal/span"

// Node is any syntax node.
type Node interface {
	Span() span.Span
	node()
}

// Stmt is a statement in a template, block, or file.
type Stmt interface {
	Node
	stmt()
}

// Decl is a statement that introduces a name.
type Decl interface {
	Stmt
	decl()
}

// Expr is an expression.
type Expr interface {
	Node
	expr()
}

// TypeNode is a type expression.
type TypeNode interface {
	Node
	typeNode()
}

// Pattern is a pattern.
type Pattern interface {
	Node
	pattern()
}

// File is one compilation unit.
type File struct {
	S       span.Span
	Package *QualName
	Imports []*Import
	Stats   []Stmt
}

func (n *File) Span() span.Span { return n.S }
func (n *File) node()           {}

// QualName is a dotted name such as a package or a type path written by the user.
type QualName struct {
	S     span.Span
	Parts []string
}

func (n *QualName) Span() span.Span { return n.S }
func (n *QualName) node()           {}

func (n *QualName) String() string {
	if n == nil || len(n.Parts) == 0 {
		return ""
	}
	s := n.Parts[0]
	for _, p := range n.Parts[1:] {
		s += "." + p
	}
	return s
}

// Import is one import clause.
// Selectors empty and Wildcard false means the last path component is the name.
type Import struct {
	S         span.Span
	Path      *QualName
	Selectors []ImportSel
	Wildcard  bool
}

func (n *Import) Span() span.Span { return n.S }
func (n *Import) node()           {}
func (n *Import) stmt()           {}

// ImportSel renames one imported member. Rename empty means the name is kept.
// A selector named "_" is a wildcard and is also recorded on Import.Wildcard.
type ImportSel struct {
	Span   span.Span
	Name   string
	Rename string
}

// Annotation is `@Name` or `@Name(args)` attached to a declaration.
type Annotation struct {
	Span span.Span
	Name string
	Args []Expr
}

// Mod is a modifier keyword.
type Mod struct {
	Span span.Span
	Kind ModKind
}

// ModKind enumerates modifiers the parser accepts.
type ModKind int

const (
	ModAbstract ModKind = iota
	ModFinal
	ModSealed
	ModOverride
	ModLazy
	ModOpaque
	ModOpen
	ModInfix
	ModInline
	ModTransparent
	ModPrivate
	ModProtected
	ModCase
	ModImplicit
)

func (k ModKind) String() string {
	switch k {
	case ModAbstract:
		return "abstract"
	case ModFinal:
		return "final"
	case ModSealed:
		return "sealed"
	case ModOverride:
		return "override"
	case ModLazy:
		return "lazy"
	case ModOpaque:
		return "opaque"
	case ModOpen:
		return "open"
	case ModInfix:
		return "infix"
	case ModInline:
		return "inline"
	case ModTransparent:
		return "transparent"
	case ModPrivate:
		return "private"
	case ModProtected:
		return "protected"
	case ModCase:
		return "case"
	case ModImplicit:
		return "implicit"
	default:
		return "modifier"
	}
}

// HasMod reports whether mods contains k.
func HasMod(mods []Mod, k ModKind) bool {
	for _, m := range mods {
		if m.Kind == k {
			return true
		}
	}
	return false
}

// Variance is the variance of a type parameter.
type Variance int

const (
	Invariant Variance = iota
	Covariant
	Contravariant
)

func (v Variance) String() string {
	switch v {
	case Covariant:
		return "+"
	case Contravariant:
		return "-"
	default:
		return ""
	}
}

// TypeParam is a type parameter. Params nonempty means a higher-kinded parameter,
// where each nested parameter is a kind argument (`F[_]` has one anonymous parameter).
type TypeParam struct {
	S        span.Span
	Name     string
	Variance Variance
	Params   []*TypeParam
	Lo       TypeNode
	Hi       TypeNode
}

func (n *TypeParam) Span() span.Span { return n.S }
func (n *TypeParam) node()           {}

// Param is a value parameter. Mods may contain val or be empty.
// Val and Var are also recorded explicitly because they affect field generation.
type Param struct {
	S     span.Span
	Name  string
	Type  TypeNode
	IsVal bool
	IsVar bool
}

func (n *Param) Span() span.Span { return n.S }
func (n *Param) node()           {}

// ParamList is one parameter clause, optionally a using clause.
type ParamList struct {
	S      span.Span
	Using  bool
	Params []*Param
}

func (n *ParamList) Span() span.Span { return n.S }
func (n *ParamList) node()           {}

// Parent is a parent type in an extends clause, with optional constructor arguments.
type Parent struct {
	S       span.Span
	Type    TypeNode
	Args    []Expr
	HasArgs bool
}

func (n *Parent) Span() span.Span { return n.S }
func (n *Parent) node()           {}

// Template is a class, trait, object, enum, extension, or anonymous-class body.
type Template struct {
	S        span.Span
	SelfName string
	SelfType TypeNode
	Stats    []Stmt
	Indented bool
}

func (n *Template) Span() span.Span { return n.S }
func (n *Template) node()           {}

// ExprStmt wraps an expression used as a statement.
type ExprStmt struct {
	S span.Span
	X Expr
}

func (n *ExprStmt) Span() span.Span { return n.S }
func (n *ExprStmt) node()           {}
func (n *ExprStmt) stmt()           {}
