// Package vm executes Fathom bytecode.
//
// Values that point at the heap are integer references. A precise mark-sweep
// collector traces frames, the operand stack, and globals, so the machine does
// not depend on the host garbage collector to retain Fathom objects.
package vm

import (
	"fmt"
	"strings"

	"github.com/Aneesh495/fathom/internal/bc"
	"github.com/Aneesh495/fathom/internal/compile"
	"github.com/Aneesh495/fathom/internal/types"
)

type tag byte

const (
	tUnit tag = iota
	tBool
	tInt
	tFloat
	tString
	tObj
	tClos
)

type val struct {
	tag tag
	i   int64
	f   float64
	b   bool
	s   string
	ref int
}

func (v val) String() string {
	switch v.tag {
	case tUnit:
		return "()"
	case tBool:
		if v.b {
			return "true"
		}
		return "false"
	case tInt:
		return fmt.Sprintf("%d", v.i)
	case tFloat:
		return fmt.Sprintf("%g", v.f)
	case tString:
		return v.s
	case tObj:
		return "obj"
	case tClos:
		return "<function>"
	default:
		return "?"
	}
}

type object struct {
	class  int
	fields []val
	fn     int
	caps   []val
	mark   bool
	kind   int // 0 instance, 1 closure
}

type frame struct {
	fn     *compile.Func
	pc     int
	locals []val
	stack  []val
}

// Machine is one execution of a program.
type Machine struct {
	prog      *compile.Program
	heap      []*object
	free      []int
	globals   []val
	out       strings.Builder
	callStack []string
	allocs    int
	nextGC    int
	depth     int
	frames    []*frame
	fault     error
}

// Result is the value of main and the captured standard output.
type Result struct {
	Value  string
	Output string
}

// Run executes prog.Main.
func Run(prog *compile.Program) (Result, error) {
	m := &Machine{prog: prog, nextGC: 32}
	m.heap = []*object{nil}
	m.globals = make([]val, prog.Globals)
	for i, sym := range prog.GlobalSym {
		if sym == nil {
			continue
		}
		// Class id was recorded on the program. Find it by scanning.
		id := 0
		for _, cl := range prog.Classes {
			if cl != nil && cl.Sym == sym {
				id = cl.ID
				break
			}
		}
		ref := m.alloc(id, false, 0, nil)
		m.globals[i] = val{tag: tObj, ref: ref}
	}
	if prog.Main < 0 || prog.Main >= len(prog.Funcs) {
		return Result{}, fmt.Errorf("no main")
	}
	v, err := m.call(prog.Funcs[prog.Main], nil)
	if err != nil {
		return Result{Output: m.out.String()}, err
	}
	return Result{Value: m.show(v), Output: m.out.String()}, nil
}

func (m *Machine) alloc(class int, clos bool, fn int, caps []val) int {
	nfields := 0
	if class > 0 && class < len(m.prog.Classes) && m.prog.Classes[class] != nil {
		nfields = m.prog.Classes[class].NFields
	}
	o := &object{class: class, fields: make([]val, nfields), fn: fn, caps: caps}
	if clos {
		o.kind = 1
	}
	var ref int
	if n := len(m.free); n > 0 {
		ref = m.free[n-1]
		m.free = m.free[:n-1]
		m.heap[ref] = o
	} else {
		ref = len(m.heap)
		m.heap = append(m.heap, o)
	}
	m.allocs++
	return ref
}

func (m *Machine) maybeGC() {
	if m.allocs >= m.nextGC {
		m.gc()
		m.nextGC = m.allocs + 32
	}
}

func (m *Machine) call(fn *compile.Func, args []val) (val, error) {
	if fn == nil {
		return val{}, fmt.Errorf("nil function")
	}
	if m.depth > 100000 {
		return val{}, m.error("stack overflow")
	}
	locals := make([]val, fn.Locals)
	for i := 0; i < len(args) && i < len(locals); i++ {
		locals[i] = args[i]
	}
	fr := &frame{fn: fn, locals: locals}
	m.frames = append(m.frames, fr)
	m.depth++
	m.callStack = append(m.callStack, fn.Name)
	defer func() {
		m.depth--
		m.frames = m.frames[:len(m.frames)-1]
		m.callStack = m.callStack[:len(m.callStack)-1]
	}()
	for {
		if fr.pc < 0 || fr.pc >= len(fn.Code) {
			return val{}, m.error("pc out of range")
		}
		in, err := bc.Decode(fn.Code, fr.pc)
		if err != nil {
			return val{}, m.error(err.Error())
		}
		fr.pc = in.Next
		switch in.Op {
		case bc.OpHalt, bc.OpReturn:
			if len(fr.stack) == 0 {
				return val{tag: tUnit}, nil
			}
			return fr.stack[len(fr.stack)-1], nil
		case bc.OpNop:
		case bc.OpConst:
			fr.push(m.constVal(fn, in.A))
		case bc.OpUnit:
			fr.push(val{tag: tUnit})
		case bc.OpNull:
			fr.push(val{})
		case bc.OpPop:
			fr.pop()
		case bc.OpDup:
			v := fr.pop()
			fr.push(v)
			fr.push(v)
		case bc.OpLoad:
			if in.A >= len(fr.locals) {
				return val{}, m.error("bad local")
			}
			fr.push(fr.locals[in.A])
		case bc.OpStore:
			v := fr.pop()
			if in.A < len(fr.locals) {
				fr.locals[in.A] = v
			}
		case bc.OpAdd, bc.OpSub, bc.OpMul, bc.OpDiv, bc.OpMod:
			b, a := fr.pop(), fr.pop()
			r, err := arith(in.Op, a, b)
			if err != nil {
				return val{}, m.error(err.Error())
			}
			fr.push(r)
		case bc.OpNeg:
			a := fr.pop()
			if a.tag == tFloat {
				fr.push(val{tag: tFloat, f: -a.f})
			} else {
				fr.push(val{tag: tInt, i: -a.i})
			}
		case bc.OpEq:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tBool, b: m.equal(a, b)})
		case bc.OpNe:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tBool, b: !m.equal(a, b)})
		case bc.OpLt, bc.OpLe, bc.OpGt, bc.OpGe:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tBool, b: cmpOp(in.Op, a, b)})
		case bc.OpNot:
			a := fr.pop()
			fr.push(val{tag: tBool, b: !truth(a)})
		case bc.OpBitAnd:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tInt, i: a.i & b.i})
		case bc.OpBitOr:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tInt, i: a.i | b.i})
		case bc.OpBitXor:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tInt, i: a.i ^ b.i})
		case bc.OpBitNot:
			a := fr.pop()
			fr.push(val{tag: tInt, i: ^a.i})
		case bc.OpShl:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tInt, i: a.i << uint(b.i)})
		case bc.OpShr:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tInt, i: a.i >> uint(b.i)})
		case bc.OpJump:
			fr.pc = in.Next + in.A
		case bc.OpJumpIf:
			if truth(fr.pop()) {
				fr.pc = in.Next + in.A
			}
		case bc.OpJumpIfNot:
			if !truth(fr.pop()) {
				fr.pc = in.Next + in.A
			}
		case bc.OpNew:
			m.maybeGC()
			ref := m.alloc(in.A, false, 0, nil)
			fr.push(val{tag: tObj, ref: ref})
		case bc.OpGetField:
			obj := fr.pop()
			fr.push(m.getField(obj, in.A))
		case bc.OpPutField:
			v := fr.pop()
			obj := fr.pop()
			m.putField(obj, in.A, v)
		case bc.OpGlobal:
			if in.A >= 0 && in.A < len(m.globals) {
				fr.push(m.globals[in.A])
			} else {
				fr.push(val{})
			}
		case bc.OpCall:
			args := fr.popN(in.B)
			callee := m.prog.Funcs[in.A]
			v, err := m.call(callee, args)
			if err != nil {
				return val{}, err
			}
			fr.push(v)
		case bc.OpCallVirt:
			name := ""
			if in.A >= 0 && in.A < len(fn.Consts) {
				name = fn.Consts[in.A].S
			}
			args := fr.popN(in.B)
			if len(args) == 0 || args[0].tag != tObj {
				return val{}, m.error("virtual call without a receiver")
			}
			obj := m.obj(args[0].ref)
			if obj == nil {
				return val{}, m.error("null receiver")
			}
			id := m.lookup(obj.class, name)
			if id < 0 {
				return val{}, m.error("missing method " + name)
			}
			v, err := m.call(m.prog.Funcs[id], args)
			if err != nil {
				return val{}, err
			}
			fr.push(v)
		case bc.OpCallClos:
			args := fr.popN(in.A)
			clos := fr.pop()
			o := m.obj(clos.ref)
			if o == nil || o.kind != 1 {
				return val{}, m.error("call of non-function")
			}
			full := make([]val, 0, len(args)+len(o.caps))
			full = append(full, args...)
			full = append(full, o.caps...)
			v, err := m.call(m.prog.Funcs[o.fn], full)
			if err != nil {
				return val{}, err
			}
			fr.push(v)
		case bc.OpClosure:
			m.maybeGC()
			n := in.B
			caps := fr.popN(n)
			ref := m.alloc(0, true, in.A, caps)
			fr.push(val{tag: tClos, ref: ref})
		case bc.OpClassEq:
			obj := fr.pop()
			ok := false
			if o := m.obj(obj.ref); o != nil && obj.tag == tObj {
				ok = o.class == in.A
			}
			fr.push(val{tag: tBool, b: ok})
		case bc.OpPrint:
			v := fr.pop()
			m.out.WriteString(m.show(v))
		case bc.OpPrintln:
			v := fr.pop()
			m.out.WriteString(m.show(v))
			m.out.WriteByte('\n')
		case bc.OpConcat:
			b, a := fr.pop(), fr.pop()
			fr.push(val{tag: tString, s: m.show(a) + m.show(b)})
		case bc.OpTrap:
			msg := "trap"
			if in.A >= 0 && in.A < len(fn.Consts) {
				msg = fn.Consts[in.A].S
			}
			return val{}, m.error(msg)
		default:
			return val{}, m.error("bad opcode")
		}
		if m.fault != nil {
			return val{}, m.fault
		}
	}
}

func (m *Machine) error(msg string) error {
	var b strings.Builder
	b.WriteString("runtime error: ")
	b.WriteString(msg)
	for i := len(m.callStack) - 1; i >= 0; i-- {
		b.WriteString("\n  at ")
		b.WriteString(m.callStack[i])
	}
	return fmt.Errorf("%s", b.String())
}

func (f *frame) push(v val) { f.stack = append(f.stack, v) }

func (f *frame) pop() val {
	n := len(f.stack)
	if n == 0 {
		return val{}
	}
	v := f.stack[n-1]
	f.stack = f.stack[:n-1]
	return v
}

func (f *frame) popN(n int) []val {
	out := make([]val, n)
	for i := n - 1; i >= 0; i-- {
		out[i] = f.pop()
	}
	return out
}

func (m *Machine) obj(ref int) *object {
	if ref <= 0 || ref >= len(m.heap) {
		return nil
	}
	return m.heap[ref]
}

func (m *Machine) getField(v val, idx int) val {
	o := m.obj(v.ref)
	if o == nil || v.tag != tObj || idx < 0 || idx >= len(o.fields) {
		m.fault = m.error("bad field")
		return val{}
	}
	return o.fields[idx]
}

func (m *Machine) putField(obj val, idx int, v val) {
	o := m.obj(obj.ref)
	if o == nil || idx < 0 || idx >= len(o.fields) {
		m.fault = m.error("bad field write")
		return
	}
	o.fields[idx] = v
}

func (m *Machine) constVal(fn *compile.Func, i int) val {
	if i < 0 || i >= len(fn.Consts) {
		return val{}
	}
	k := fn.Consts[i]
	switch k.Tag {
	case compile.ConstInt:
		return val{tag: tInt, i: k.I}
	case compile.ConstFloat:
		return val{tag: tFloat, f: k.F}
	case compile.ConstBool:
		return val{tag: tBool, b: k.B}
	default:
		return val{tag: tString, s: k.S}
	}
}

func (m *Machine) lookup(classID int, name string) int {
	if classID <= 0 || classID >= len(m.prog.Classes) {
		return -1
	}
	cl := m.prog.Classes[classID]
	ids := cl.Lin
	if len(ids) == 0 {
		ids = []int{classID}
	}
	for _, id := range ids {
		if id < 0 || id >= len(m.prog.Classes) || m.prog.Classes[id] == nil {
			continue
		}
		if fn, ok := m.prog.Classes[id].Methods[name]; ok {
			return fn
		}
	}
	return -1
}

func truth(v val) bool { return v.tag == tBool && v.b }

func arith(op bc.Op, a, b val) (val, error) {
	if a.tag == tFloat || b.tag == tFloat {
		af, bf := asFloat(a), asFloat(b)
		switch op {
		case bc.OpAdd:
			return val{tag: tFloat, f: af + bf}, nil
		case bc.OpSub:
			return val{tag: tFloat, f: af - bf}, nil
		case bc.OpMul:
			return val{tag: tFloat, f: af * bf}, nil
		case bc.OpDiv:
			if bf == 0 {
				return val{}, fmt.Errorf("division by zero")
			}
			return val{tag: tFloat, f: af / bf}, nil
		default:
			return val{}, fmt.Errorf("bad float op")
		}
	}
	switch op {
	case bc.OpAdd:
		return val{tag: tInt, i: a.i + b.i}, nil
	case bc.OpSub:
		return val{tag: tInt, i: a.i - b.i}, nil
	case bc.OpMul:
		return val{tag: tInt, i: a.i * b.i}, nil
	case bc.OpDiv:
		if b.i == 0 {
			return val{}, fmt.Errorf("division by zero")
		}
		return val{tag: tInt, i: a.i / b.i}, nil
	case bc.OpMod:
		if b.i == 0 {
			return val{}, fmt.Errorf("division by zero")
		}
		return val{tag: tInt, i: a.i % b.i}, nil
	default:
		return val{}, fmt.Errorf("bad arithmetic")
	}
}

func asFloat(v val) float64 {
	if v.tag == tFloat {
		return v.f
	}
	return float64(v.i)
}

func cmpOp(op bc.Op, a, b val) bool {
	var c int
	if a.tag == tFloat || b.tag == tFloat {
		af, bf := asFloat(a), asFloat(b)
		switch {
		case af < bf:
			c = -1
		case af > bf:
			c = 1
		}
	} else {
		switch {
		case a.i < b.i:
			c = -1
		case a.i > b.i:
			c = 1
		}
	}
	switch op {
	case bc.OpLt:
		return c < 0
	case bc.OpLe:
		return c <= 0
	case bc.OpGt:
		return c > 0
	default:
		return c >= 0
	}
}

func (m *Machine) equal(a, b val) bool {
	if a.tag != b.tag {
		return false
	}
	switch a.tag {
	case tUnit:
		return true
	case tBool:
		return a.b == b.b
	case tInt:
		return a.i == b.i
	case tFloat:
		return a.f == b.f
	case tString:
		return a.s == b.s
	case tObj:
		if a.ref == b.ref {
			return true
		}
		oa, ob := m.obj(a.ref), m.obj(b.ref)
		if oa == nil || ob == nil || oa.class != ob.class {
			return false
		}
		cl := m.prog.Classes[oa.class]
		if cl == nil || cl.Sym == nil || !cl.Sym.Has(types.FlagCase) {
			return false
		}
		if len(oa.fields) != len(ob.fields) {
			return false
		}
		for i := range oa.fields {
			if !m.equal(oa.fields[i], ob.fields[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (m *Machine) show(v val) string {
	switch v.tag {
	case tUnit:
		return "()"
	case tBool:
		if v.b {
			return "true"
		}
		return "false"
	case tInt:
		return fmt.Sprintf("%d", v.i)
	case tFloat:
		return fmt.Sprintf("%g", v.f)
	case tString:
		return v.s
	case tClos:
		return "<function>"
	case tObj:
		o := m.obj(v.ref)
		if o == nil {
			return "null"
		}
		cl := (*compile.Class)(nil)
		if o.class > 0 && o.class < len(m.prog.Classes) {
			cl = m.prog.Classes[o.class]
		}
		if cl == nil {
			return "null"
		}
		if cl.Sym != nil && cl.Sym.Has(types.FlagCase) && cl.Sym.Kind == types.SymClass && len(o.fields) > 0 {
			// Kind class is 1. Print fields.
			var b strings.Builder
			b.WriteString(cl.Name)
			b.WriteByte('(')
			for i, f := range o.fields {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(m.show(f))
			}
			b.WriteByte(')')
			return b.String()
		}
		return cl.Name
	default:
		return "null"
	}
}
