package eval

import "github.com/Aneesh495/fathom/internal/types"

func (m *Machine) builtin(op types.Builtin, recv Value, args []Value) Value {
	arg := func(i int) Value {
		if i < len(args) {
			return args[i]
		}
		return Value{}
	}
	switch op {
	case types.BuiltinAdd:
		if recv.Tag == VFloat || arg(0).Tag == VFloat {
			return Value{Tag: VFloat, F: asFloat(recv) + asFloat(arg(0))}
		}
		return integer(recv.I + arg(0).I)
	case types.BuiltinSub:
		if recv.Tag == VFloat || arg(0).Tag == VFloat {
			return Value{Tag: VFloat, F: asFloat(recv) - asFloat(arg(0))}
		}
		return integer(recv.I - arg(0).I)
	case types.BuiltinMul:
		if recv.Tag == VFloat || arg(0).Tag == VFloat {
			return Value{Tag: VFloat, F: asFloat(recv) * asFloat(arg(0))}
		}
		return integer(recv.I * arg(0).I)
	case types.BuiltinDiv:
		if recv.Tag == VFloat || arg(0).Tag == VFloat {
			d := asFloat(arg(0))
			if d == 0 {
				return m.fail("division by zero")
			}
			return Value{Tag: VFloat, F: asFloat(recv) / d}
		}
		if arg(0).I == 0 {
			return m.fail("division by zero")
		}
		return integer(recv.I / arg(0).I)
	case types.BuiltinMod:
		if arg(0).I == 0 {
			return m.fail("division by zero")
		}
		return integer(recv.I % arg(0).I)
	case types.BuiltinNeg:
		if recv.Tag == VFloat {
			return Value{Tag: VFloat, F: -recv.F}
		}
		return integer(-recv.I)
	case types.BuiltinEq:
		return truth(equal(recv, arg(0)))
	case types.BuiltinNe:
		return truth(!equal(recv, arg(0)))
	case types.BuiltinLt:
		return truth(cmp(recv, arg(0)) < 0)
	case types.BuiltinLe:
		return truth(cmp(recv, arg(0)) <= 0)
	case types.BuiltinGt:
		return truth(cmp(recv, arg(0)) > 0)
	case types.BuiltinGe:
		return truth(cmp(recv, arg(0)) >= 0)
	case types.BuiltinNot:
		return truth(!recv.Truth())
	case types.BuiltinBitAnd:
		return integer(recv.I & arg(0).I)
	case types.BuiltinBitOr:
		return integer(recv.I | arg(0).I)
	case types.BuiltinBitXor:
		return integer(recv.I ^ arg(0).I)
	case types.BuiltinBitNot:
		return integer(^recv.I)
	case types.BuiltinShl:
		return integer(recv.I << uint(arg(0).I))
	case types.BuiltinShr:
		return integer(recv.I >> uint(arg(0).I))
	case types.BuiltinConcat:
		return str(recv.String() + arg(0).String())
	case types.BuiltinToString:
		return str(recv.String())
	case types.BuiltinLength:
		if recv.Tag == VString {
			return integer(int64(len([]rune(recv.S))))
		}
		return m.fail("length of non-string")
	case types.BuiltinPrint:
		m.out.Write([]byte(arg(0).String()))
		return unit()
	case types.BuiltinPrintln:
		m.out.Write([]byte(arg(0).String() + "\n"))
		return unit()
	case types.BuiltinNone:
		return recv
	default:
		return m.fail("unimplemented primitive")
	}
}

func asFloat(v Value) float64 {
	if v.Tag == VFloat {
		return v.F
	}
	return float64(v.I)
}

func equal(a, b Value) bool {
	if a.Tag != b.Tag {
		return false
	}
	switch a.Tag {
	case VUnit, VNull:
		return true
	case VBool:
		return a.B == b.B
	case VInt:
		return a.I == b.I
	case VFloat:
		return a.F == b.F
	case VChar:
		return a.C == b.C
	case VString:
		return a.S == b.S
	case VObj:
		if a.Obj == nil || b.Obj == nil {
			return a.Obj == b.Obj
		}
		if a.Obj == b.Obj {
			return true
		}
		if a.Obj.Class != b.Obj.Class || !a.Obj.Class.Has(types.FlagCase) {
			return false
		}
		if len(a.Obj.Fields) != len(b.Obj.Fields) {
			return false
		}
		for i := range a.Obj.Fields {
			if !equal(a.Obj.Fields[i], b.Obj.Fields[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func cmp(a, b Value) int {
	if a.Tag == VFloat || b.Tag == VFloat {
		af, bf := asFloat(a), asFloat(b)
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	switch {
	case a.I < b.I:
		return -1
	case a.I > b.I:
		return 1
	default:
		return 0
	}
}
