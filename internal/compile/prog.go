// Package compile lowers a typechecked Fathom program to bytecode.
//
// The lowering is deliberately close to the reference evaluator: the same
// symbol identities, the same field order, and the same calling convention.
// A differential test runs both and compares results.
package compile

import "github.com/Aneesh495/fathom/internal/types"

// ConstTag discriminates a constant pool entry.
type ConstTag int

const (
	ConstInt ConstTag = iota
	ConstFloat
	ConstString
	ConstBool
)

// Const is one constant.
type Const struct {
	Tag ConstTag
	I   int64
	F   float64
	S   string
	B   bool
}

// Func is one bytecode function.
type Func struct {
	Name     string
	Locals   int
	Code     []byte
	Consts   []Const
	MaxStack int
	Sym      *types.Symbol
}

// Class is the VM view of a class, object, or given instance.
type Class struct {
	Name      string
	ID        int
	NFields   int
	Lin       []int
	Methods   map[string]int
	Singleton bool
	Sym       *types.Symbol
}

// Program is a closed bytecode image.
type Program struct {
	Funcs   []*Func
	Classes []*Class
	Main    int
	Globals int
	// GlobalSym maps a global slot to the symbol it materializes.
	GlobalSym []*types.Symbol
}

// Disasm is filled in by the driver when a test wants to see code.
func (p *Program) FuncNamed(name string) *Func {
	for _, f := range p.Funcs {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}
