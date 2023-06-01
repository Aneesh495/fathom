// Package eval executes a typechecked Fathom program by walking its syntax.
//
// This walker is the specification of the language. The bytecode machine in
// package vm is required to agree with it on every program the driver runs.
package eval

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/types"
)

// Tag discriminates a runtime value.
type Tag int

const (
	VUnit Tag = iota
	VBool
	VInt
	VFloat
	VChar
	VString
	VNull
	VObj
	VFun
)

// Object is a class instance or a singleton module.
type Object struct {
	Class  *types.Symbol
	Fields []Value
}

// Fun is a closure or a method value.
type Fun struct {
	Sym     *types.Symbol
	Lam     *ast.LambdaExpr
	Env     *env
	Params  []*types.Symbol
	This    Value
	HasThis bool
}

// Value is one runtime value. Unused fields are zero.
type Value struct {
	Tag Tag
	I   int64
	F   float64
	B   bool
	S   string
	C   rune
	Obj *Object
	Fun *Fun
}

func (v Value) String() string {
	switch v.Tag {
	case VUnit:
		return "()"
	case VBool:
		if v.B {
			return "true"
		}
		return "false"
	case VInt:
		return strconv.FormatInt(v.I, 10)
	case VFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case VChar:
		return strconv.QuoteRune(v.C)
	case VString:
		return v.S
	case VNull:
		return "null"
	case VFun:
		if v.Fun != nil && v.Fun.Sym != nil {
			return "<" + v.Fun.Sym.Name + ">"
		}
		return "<function>"
	case VObj:
		if v.Obj == nil || v.Obj.Class == nil {
			return "null"
		}
		if v.Obj.Class.Has(types.FlagCase) && v.Obj.Class.Kind == types.SymClass && len(v.Obj.Fields) > 0 {
			var b strings.Builder
			b.WriteString(v.Obj.Class.Name)
			b.WriteByte('(')
			for i, f := range v.Obj.Fields {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(f.String())
			}
			b.WriteByte(')')
			return b.String()
		}
		return v.Obj.Class.Name
	default:
		return "?"
	}
}

func (v Value) Truth() bool {
	return v.Tag == VBool && v.B
}

type env struct {
	vals   map[*types.Symbol]Value
	parent *env
}

func newEnv(parent *env) *env {
	return &env{vals: map[*types.Symbol]Value{}, parent: parent}
}

func (e *env) get(s *types.Symbol) (Value, bool) {
	for e != nil {
		if v, ok := e.vals[s]; ok {
			return v, true
		}
		e = e.parent
	}
	return Value{}, false
}

func (e *env) set(s *types.Symbol, v Value) {
	if e == nil || s == nil {
		return
	}
	if e.vals == nil {
		e.vals = map[*types.Symbol]Value{}
	}
	e.vals[s] = v
}

// Error is a runtime failure with a call stack.
type Error struct {
	Msg   string
	Stack []string
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("runtime error: ")
	b.WriteString(e.Msg)
	for i := len(e.Stack) - 1; i >= 0; i-- {
		b.WriteString("\n  at ")
		b.WriteString(e.Stack[i])
	}
	return b.String()
}

// Output is where println writes. Tests replace it.
type Output struct {
	w io.Writer
	b strings.Builder
}

func (o *Output) Write(p []byte) (int, error) {
	if o.w != nil {
		return o.w.Write(p)
	}
	return o.b.Write(p)
}

func (o *Output) String() string { return o.b.String() }

func unit() Value { return Value{Tag: VUnit} }

func truth(b bool) Value { return Value{Tag: VBool, B: b} }

func integer(i int64) Value { return Value{Tag: VInt, I: i} }

func str(s string) Value { return Value{Tag: VString, S: s} }

func showQuoted(v Value) string {
	if v.Tag == VString {
		return strconv.Quote(v.S)
	}
	return v.String()
}

func binName(op types.Builtin) string {
	return fmt.Sprintf("builtin(%d)", op)
}
