// Package bc is the bytecode of the Fathom machine.
//
// Instructions are a stack machine with explicit locals. Jumps are relative
// and signed. The encoding table is the single source of truth for the
// assembler, the verifier, and the disassembler.
package bc

// Op is one instruction opcode.
type Op byte

const (
	OpHalt Op = iota
	OpNop
	OpConst
	OpUnit
	OpNull
	OpPop
	OpDup
	OpLoad
	OpStore
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	OpNot
	OpJump
	OpJumpIf
	OpJumpIfNot
	OpNew
	OpGetField
	OpPutField
	OpCall
	OpCallVirt
	OpReturn
	OpPrint
	OpPrintln
	OpConcat
	OpClosure
	OpClassEq
	OpBitAnd
	OpBitOr
	OpBitXor
	OpBitNot
	OpShl
	OpShr
	OpGlobal
	OpTrap
	OpCallClos
)

// Imm is the immediate width of an opcode.
type Imm int

const (
	ImmNone Imm = iota
	ImmU8
	ImmU16
	ImmI16
	ImmU16U8 // two immediates: u16 then u8
	ImmU8U8
)

// Info describes one opcode for tools that walk code without a switch.
type Info struct {
	Op   Op
	Name string
	Imm  Imm
}

// Catalog is every opcode in numeric order.
var Catalog = [...]Info{
	{OpHalt, "halt", ImmNone},
	{OpNop, "nop", ImmNone},
	{OpConst, "const", ImmU16},
	{OpUnit, "unit", ImmNone},
	{OpNull, "null", ImmNone},
	{OpPop, "pop", ImmNone},
	{OpDup, "dup", ImmNone},
	{OpLoad, "load", ImmU16},
	{OpStore, "store", ImmU16},
	{OpAdd, "add", ImmNone},
	{OpSub, "sub", ImmNone},
	{OpMul, "mul", ImmNone},
	{OpDiv, "div", ImmNone},
	{OpMod, "mod", ImmNone},
	{OpNeg, "neg", ImmNone},
	{OpEq, "eq", ImmNone},
	{OpNe, "ne", ImmNone},
	{OpLt, "lt", ImmNone},
	{OpLe, "le", ImmNone},
	{OpGt, "gt", ImmNone},
	{OpGe, "ge", ImmNone},
	{OpNot, "not", ImmNone},
	{OpJump, "jump", ImmI16},
	{OpJumpIf, "jumpif", ImmI16},
	{OpJumpIfNot, "jumpifnot", ImmI16},
	{OpNew, "new", ImmU16},
	{OpGetField, "getfield", ImmU16},
	{OpPutField, "putfield", ImmU16},
	{OpCall, "call", ImmU16U8},
	{OpCallVirt, "callvirt", ImmU16U8},
	{OpReturn, "return", ImmNone},
	{OpPrint, "print", ImmNone},
	{OpPrintln, "println", ImmNone},
	{OpConcat, "concat", ImmNone},
	{OpClosure, "closure", ImmU16U8},
	{OpClassEq, "classeq", ImmU16},
	{OpBitAnd, "bitand", ImmNone},
	{OpBitOr, "bitor", ImmNone},
	{OpBitXor, "bitxor", ImmNone},
	{OpBitNot, "bitnot", ImmNone},
	{OpShl, "shl", ImmNone},
	{OpShr, "shr", ImmNone},
	{OpGlobal, "global", ImmU16},
	{OpTrap, "trap", ImmU16},
	{OpCallClos, "callclos", ImmU8},
}

// Lookup returns the description of op.
func Lookup(op Op) Info {
	if int(op) < len(Catalog) {
		return Catalog[op]
	}
	return Info{Op: op, Name: "?", Imm: ImmNone}
}

// ImmSize is the number of immediate bytes.
func ImmSize(imm Imm) int {
	switch imm {
	case ImmNone:
		return 0
	case ImmU8:
		return 1
	case ImmU16, ImmI16:
		return 2
	case ImmU16U8:
		return 3
	case ImmU8U8:
		return 2
	default:
		return 0
	}
}
