package ast

import "github.com/Aneesh495/fathom/internal/span"

// Ident is a name. Quoted is true when the user wrote backticks.
type Ident struct {
	S      span.Span
	Name   string
	Quoted bool
}

func (n *Ident) Span() span.Span { return n.S }
func (n *Ident) node()           {}
func (n *Ident) expr()           {}

// LiteralExpr is a constant.
type LiteralExpr struct {
	S    span.Span
	Kind LitKind
	Text string
}

// LitKind classifies literals. Text is the decoded value, not the source spelling.
// Int and float texts still include their radix prefix so the checker can reparse them.
type LitKind int

const (
	LitInt LitKind = iota
	LitFloat
	LitString
	LitChar
	LitBool
	LitNull
	LitUnit
)

func (n *LiteralExpr) Span() span.Span { return n.S }
func (n *LiteralExpr) node()           {}
func (n *LiteralExpr) expr()           {}

// ParenExpr preserves explicit parentheses for the formatter.
type ParenExpr struct {
	S span.Span
	X Expr
}

func (n *ParenExpr) Span() span.Span { return n.S }
func (n *ParenExpr) node()           {}
func (n *ParenExpr) expr()           {}

// TupleExpr is `(a, b, ...)`. A one-element parenthesized expression is ParenExpr.
type TupleExpr struct {
	S     span.Span
	Elems []Expr
}

func (n *TupleExpr) Span() span.Span { return n.S }
func (n *TupleExpr) node()           {}
func (n *TupleExpr) expr()           {}

// BlockExpr is `{ statements }`. The value is the last expression, or Unit.
type BlockExpr struct {
	S     span.Span
	Stats []Stmt
}

func (n *BlockExpr) Span() span.Span { return n.S }
func (n *BlockExpr) node()           {}
func (n *BlockExpr) expr()           {}

// IfExpr is `if cond then a else b`. Else may be nil.
type IfExpr struct {
	S          span.Span
	Cond, Then Expr
	Else       Expr
}

func (n *IfExpr) Span() span.Span { return n.S }
func (n *IfExpr) node()           {}
func (n *IfExpr) expr()           {}

// MatchExpr is `scrut match { cases }`.
type MatchExpr struct {
	S     span.Span
	Scrut Expr
	Cases []*CaseClause
}

func (n *MatchExpr) Span() span.Span { return n.S }
func (n *MatchExpr) node()           {}
func (n *MatchExpr) expr()           {}

// CaseClause is one case of a match or a catch.
type CaseClause struct {
	S     span.Span
	Pat   Pattern
	Guard Expr
	Body  Expr
}

func (n *CaseClause) Span() span.Span { return n.S }
func (n *CaseClause) node()           {}

// WhileExpr is `while cond do body`.
type WhileExpr struct {
	S          span.Span
	Cond, Body Expr
}

func (n *WhileExpr) Span() span.Span { return n.S }
func (n *WhileExpr) node()           {}
func (n *WhileExpr) expr()           {}

// DoWhileExpr is `do body while cond`.
type DoWhileExpr struct {
	S          span.Span
	Body, Cond Expr
}

func (n *DoWhileExpr) Span() span.Span { return n.S }
func (n *DoWhileExpr) node()           {}
func (n *DoWhileExpr) expr()           {}

// ForExpr is a for comprehension. Yield false means `do`.
type ForExpr struct {
	S     span.Span
	Enums []*Enumerator
	Body  Expr
	Yield bool
}

func (n *ForExpr) Span() span.Span { return n.S }
func (n *ForExpr) node()           {}
func (n *ForExpr) expr()           {}

// Enumerator is a generator (`pat <- expr`), a binding (`pat = expr`), or a guard.
type Enumerator struct {
	S     span.Span
	Pat   Pattern
	Expr  Expr
	Guard Expr
	IsGen bool
}

func (n *Enumerator) Span() span.Span { return n.S }
func (n *Enumerator) node()           {}

// TryExpr is `try body catch { cases } finally expr`.
type TryExpr struct {
	S       span.Span
	Body    Expr
	Catches []*CaseClause
	Finally Expr
}

func (n *TryExpr) Span() span.Span { return n.S }
func (n *TryExpr) node()           {}
func (n *TryExpr) expr()           {}

// ThrowExpr is `throw expr`.
type ThrowExpr struct {
	S span.Span
	X Expr
}

func (n *ThrowExpr) Span() span.Span { return n.S }
func (n *ThrowExpr) node()           {}
func (n *ThrowExpr) expr()           {}

// ReturnExpr is `return` or `return expr`.
type ReturnExpr struct {
	S span.Span
	X Expr
}

func (n *ReturnExpr) Span() span.Span { return n.S }
func (n *ReturnExpr) node()           {}
func (n *ReturnExpr) expr()           {}

// AssignExpr is `lhs = rhs` or a compound assignment. Op is "=" or the full operator such as "+=".
type AssignExpr struct {
	S        span.Span
	LHS, RHS Expr
	Op       string
}

func (n *AssignExpr) Span() span.Span { return n.S }
func (n *AssignExpr) node()           {}
func (n *AssignExpr) expr()           {}

// LambdaExpr is a function literal. Parameter types may be nil when the expected type supplies them.
type LambdaExpr struct {
	S      span.Span
	Params []*Param
	Body   Expr
}

func (n *LambdaExpr) Span() span.Span { return n.S }
func (n *LambdaExpr) node()           {}
func (n *LambdaExpr) expr()           {}

// SelectExpr is `expr.name`.
type SelectExpr struct {
	S    span.Span
	X    Expr
	Name string
}

func (n *SelectExpr) Span() span.Span { return n.S }
func (n *SelectExpr) node()           {}
func (n *SelectExpr) expr()           {}

// ApplyExpr is a value application. TypeArgs are present for `f[T](x)`.
type ApplyExpr struct {
	S        span.Span
	Fun      Expr
	TypeArgs []TypeNode
	Args     []Expr
	Using    bool
}

func (n *ApplyExpr) Span() span.Span { return n.S }
func (n *ApplyExpr) node()           {}
func (n *ApplyExpr) expr()           {}

// TypeApplyExpr is `expr[T, U]` with no value arguments.
type TypeApplyExpr struct {
	S    span.Span
	Fun  Expr
	Args []TypeNode
}

func (n *TypeApplyExpr) Span() span.Span { return n.S }
func (n *TypeApplyExpr) node()           {}
func (n *TypeApplyExpr) expr()           {}

// InfixExpr is `left op right` before it is desugared to a method call.
type InfixExpr struct {
	S           span.Span
	Op          string
	Left, Right Expr
}

func (n *InfixExpr) Span() span.Span { return n.S }
func (n *InfixExpr) node()           {}
func (n *InfixExpr) expr()           {}

// PrefixExpr is a prefix operator.
type PrefixExpr struct {
	S  span.Span
	Op string
	X  Expr
}

func (n *PrefixExpr) Span() span.Span { return n.S }
func (n *PrefixExpr) node()           {}
func (n *PrefixExpr) expr()           {}

// ThisExpr is `this` or `Outer.this`.
type ThisExpr struct {
	S    span.Span
	Qual string
}

func (n *ThisExpr) Span() span.Span { return n.S }
func (n *ThisExpr) node()           {}
func (n *ThisExpr) expr()           {}

// SuperExpr is `super`, `super[Mix]`, or `Outer.super[Mix]`.
type SuperExpr struct {
	S    span.Span
	Qual string
	Mix  string
}

func (n *SuperExpr) Span() span.Span { return n.S }
func (n *SuperExpr) node()           {}
func (n *SuperExpr) expr()           {}

// AscribeExpr is `expr: Type`.
type AscribeExpr struct {
	S    span.Span
	X    Expr
	Type TypeNode
}

func (n *AscribeExpr) Span() span.Span { return n.S }
func (n *AscribeExpr) node()           {}
func (n *AscribeExpr) expr()           {}

// InterpExpr is an interpolated string. len(Parts) == len(Holes)+1.
type InterpExpr struct {
	S     span.Span
	Name  string
	Parts []string
	Holes []Expr
}

func (n *InterpExpr) Span() span.Span { return n.S }
func (n *InterpExpr) node()           {}
func (n *InterpExpr) expr()           {}

// NewExpr is instance creation, optionally with an anonymous template.
type NewExpr struct {
	S        span.Span
	Parents  []*Parent
	Template *Template
}

func (n *NewExpr) Span() span.Span { return n.S }
func (n *NewExpr) node()           {}
func (n *NewExpr) expr()           {}

// WildcardExpr is `_` in an expression. The type checker rejects it outside a
// context that will eventually be removed; the parser still records it.
type WildcardExpr struct {
	S span.Span
}

func (n *WildcardExpr) Span() span.Span { return n.S }
func (n *WildcardExpr) node()           {}
func (n *WildcardExpr) expr()           {}
