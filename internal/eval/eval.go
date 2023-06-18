package eval

import (
	"strconv"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/check"
	"github.com/Aneesh495/fathom/internal/types"
)

// Machine evaluates one typechecked program.
type Machine struct {
	info   *check.Info
	env    *env
	this   Value
	out    *Output
	err    *Error
	ret    bool
	retVal Value
	stack  []string
	index  map[*types.Symbol]int
	nfield map[*types.Symbol]int
	single map[*types.Symbol]Value
	depth  int
}

// Run executes main and returns its value plus captured output.
func Run(info *check.Info) (Value, string, error) {
	m := &Machine{
		info:   info,
		env:    newEnv(nil),
		out:    &Output{},
		index:  map[*types.Symbol]int{},
		nfield: map[*types.Symbol]int{},
		single: map[*types.Symbol]Value{},
	}
	m.layoutAll()
	if info.Main == nil {
		return Value{}, "", &Error{Msg: "no main method"}
	}
	v := m.callSym(info.Main, Value{}, nil, nil)
	if m.err != nil {
		return Value{}, m.out.String(), m.err
	}
	return v, m.out.String(), nil
}

func (m *Machine) layoutAll() {
	seen := map[*types.Symbol]bool{}
	var walk func(*types.Symbol)
	walk = func(s *types.Symbol) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		if s.IsClassLike() {
			m.layout(s)
		}
		for _, mem := range s.Members {
			walk(mem)
		}
		walk(s.Companion)
		if s.Scope != nil {
			for _, name := range s.Scope.Names() {
				for _, x := range s.Scope.Local(name) {
					walk(x)
				}
			}
		}
	}
	if m.info.Root != nil {
		for _, name := range m.info.Root.Names() {
			for _, s := range m.info.Root.Local(name) {
				walk(s)
			}
		}
	}
	for _, s := range m.info.Syms {
		walk(s)
	}
}

func (m *Machine) layout(sym *types.Symbol) int {
	if sym == nil {
		return 0
	}
	if n, ok := m.nfield[sym]; ok {
		return n
	}
	// Placeholder breaks cycles while parents are laid out.
	m.nfield[sym] = 0
	n := 0
	for _, p := range sym.Parents {
		ps := classOf(p)
		if ps != nil && ps != sym {
			n = m.layout(ps)
			break
		}
	}
	for _, mem := range sym.Members {
		if mem.Kind == types.SymTerm && mem.Name != "this" && mem.Owner == sym {
			m.index[mem] = n
			n++
		}
	}
	m.nfield[sym] = n
	return n
}

func classOf(t *types.Type) *types.Symbol {
	t = types.Repr(t)
	if t != nil && t.Kind == types.TyNamed {
		return t.Sym
	}
	return nil
}

func (m *Machine) fail(msg string) Value {
	if m.err == nil {
		stack := append([]string(nil), m.stack...)
		m.err = &Error{Msg: msg, Stack: stack}
	}
	return Value{}
}

func (m *Machine) expr(e ast.Expr) Value {
	if m.err != nil || m.ret || e == nil {
		return m.retVal
	}
	switch e := e.(type) {
	case *ast.LiteralExpr:
		return m.literal(e)
	case *ast.Ident:
		return m.ident(e)
	case *ast.ParenExpr:
		return m.expr(e.X)
	case *ast.TupleExpr:
		return m.tuple(e)
	case *ast.BlockExpr:
		return m.block(e)
	case *ast.IfExpr:
		c := m.expr(e.Cond)
		if m.err != nil || m.ret {
			return m.retVal
		}
		if !c.Truth() && c.Tag != VBool {
			return m.fail("condition is not a boolean")
		}
		if c.Truth() {
			return m.expr(e.Then)
		}
		if e.Else != nil {
			return m.expr(e.Else)
		}
		return unit()
	case *ast.WhileExpr:
		for {
			c := m.expr(e.Cond)
			if m.err != nil || m.ret || !c.Truth() {
				break
			}
			m.expr(e.Body)
			if m.err != nil || m.ret {
				break
			}
		}
		return unit()
	case *ast.DoWhileExpr:
		for {
			m.expr(e.Body)
			if m.err != nil || m.ret {
				break
			}
			c := m.expr(e.Cond)
			if m.err != nil || m.ret || !c.Truth() {
				break
			}
		}
		return unit()
	case *ast.AssignExpr:
		return m.assign(e)
	case *ast.PrefixExpr:
		return m.prefix(e)
	case *ast.InfixExpr:
		return m.infix(e)
	case *ast.SelectExpr:
		return m.selectExpr(e)
	case *ast.ApplyExpr:
		return m.apply(e)
	case *ast.LambdaExpr:
		return m.lambda(e)
	case *ast.AscribeExpr:
		return m.expr(e.X)
	case *ast.ThisExpr:
		return m.this
	case *ast.ReturnExpr:
		if e.X != nil {
			m.retVal = m.expr(e.X)
		} else {
			m.retVal = unit()
		}
		m.ret = true
		return m.retVal
	case *ast.ThrowExpr:
		v := m.expr(e.X)
		return m.fail("throw " + v.String())
	case *ast.MatchExpr:
		return m.match(e)
	case *ast.NewExpr:
		return m.newObj(e)
	case *ast.InterpExpr:
		return m.interp(e)
	case *ast.ForExpr:
		if rw := m.info.Rewrites[e]; rw != nil {
			return m.expr(rw)
		}
		return m.fail("for comprehension was not desugared")
	default:
		return m.fail("cannot evaluate expression")
	}
}

func (m *Machine) literal(e *ast.LiteralExpr) Value {
	switch e.Kind {
	case ast.LitInt:
		n, err := strconv.ParseInt(strip(e.Text), 0, 64)
		if err != nil {
			// Hex and binary prefixes are accepted by ParseInt with base 0
			// only for 0x. 0b needs an explicit base.
			n, err = parseInt(e.Text)
			if err != nil {
				return m.fail("bad integer")
			}
		}
		return integer(n)
	case ast.LitFloat:
		f, err := strconv.ParseFloat(strip(e.Text), 64)
		if err != nil {
			return m.fail("bad float")
		}
		return Value{Tag: VFloat, F: f}
	case ast.LitBool:
		return truth(e.Text == "true")
	case ast.LitString:
		return str(e.Text)
	case ast.LitChar:
		r := rune(0)
		for _, c := range e.Text {
			r = c
			break
		}
		return Value{Tag: VChar, C: r}
	case ast.LitUnit:
		return unit()
	case ast.LitNull:
		return Value{Tag: VNull}
	default:
		return m.fail("bad literal")
	}
}

func parseInt(text string) (int64, error) {
	s := strip(text)
	base := 10
	if len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		base = 16
		s = s[2:]
	} else if len(s) > 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B') {
		base = 2
		s = s[2:]
	}
	return strconv.ParseInt(s, base, 64)
}

func strip(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func (m *Machine) ident(e *ast.Ident) Value {
	sym := m.info.Syms[e]
	if sym == nil {
		return m.fail("unbound " + e.Name)
	}
	if v, ok := m.env.get(sym); ok {
		return v
	}
	if m.this.Obj != nil {
		if idx, ok := m.index[sym]; ok && idx < len(m.this.Obj.Fields) {
			return m.this.Obj.Fields[idx]
		}
	}
	switch sym.Kind {
	case types.SymObject:
		return m.singleton(sym)
	case types.SymMethod:
		return Value{Tag: VFun, Fun: &Fun{Sym: sym, Env: m.env}}
	case types.SymTerm:
		if sym.Has(types.FlagGiven) {
			return m.givenVal(sym)
		}
	case types.SymClass:
		return m.fail(sym.Name + " is a constructor, not a value")
	}
	return m.fail("no value for " + e.Name)
}

func (m *Machine) block(e *ast.BlockExpr) Value {
	saved := m.env
	m.env = newEnv(saved)
	var v Value
	for _, st := range e.Stats {
		v = m.stmt(st)
		if m.err != nil || m.ret {
			break
		}
	}
	m.env = saved
	return v
}

func (m *Machine) stmt(st ast.Stmt) Value {
	switch st := st.(type) {
	case *ast.ExprStmt:
		return m.expr(st.X)
	case *ast.ValDecl:
		sym := m.info.Syms[st]
		v := Value{}
		if st.Body != nil {
			v = m.expr(st.Body)
		}
		if sym != nil && m.err == nil && !m.ret {
			m.env.set(sym, v)
		}
		return unit()
	case *ast.DefDecl:
		sym := m.info.Syms[st]
		if sym != nil {
			m.env.set(sym, Value{Tag: VFun, Fun: &Fun{Sym: sym, Env: m.env}})
		}
		return unit()
	default:
		return unit()
	}
}

func (m *Machine) tuple(e *ast.TupleExpr) Value {
	var class *types.Symbol
	switch len(e.Elems) {
	case 2:
		class = m.info.Ctx.Tuple2
	case 3:
		class = m.info.Ctx.Tuple3
	case 4:
		class = m.info.Ctx.Tuple4
	default:
		return m.fail("bad tuple")
	}
	args := make([]Value, len(e.Elems))
	for i, el := range e.Elems {
		args[i] = m.expr(el)
		if m.err != nil || m.ret {
			return m.retVal
		}
	}
	return m.construct(class, args)
}

func (m *Machine) assign(e *ast.AssignExpr) Value {
	rhs := e.RHS
	if e.Op != "=" {
		base := e.Op[:len(e.Op)-1]
		rhs = &ast.InfixExpr{S: e.Span(), Op: base, Left: e.LHS, Right: e.RHS}
	}
	v := m.expr(rhs)
	if m.err != nil || m.ret {
		return m.retVal
	}
	switch lhs := e.LHS.(type) {
	case *ast.Ident:
		sym := m.info.Syms[lhs]
		if sym == nil {
			return m.fail("bad assignment")
		}
		if m.env.update(sym, v) {
			return unit()
		}
		if sym.Owner == nil || !sym.Owner.IsClassLike() {
			m.env.set(sym, v)
			return unit()
		}
		return m.setField(m.this, sym, v)
	case *ast.SelectExpr:
		recv := m.expr(lhs.X)
		sym := m.info.Syms[lhs]
		return m.setField(recv, sym, v)
	default:
		return m.fail("bad assignment")
	}
}

func (m *Machine) setField(recv Value, sym *types.Symbol, v Value) Value {
	if recv.Tag == VNull || recv.Obj == nil {
		return m.fail("assignment to null")
	}
	idx, ok := m.index[sym]
	if !ok || idx >= len(recv.Obj.Fields) {
		return m.fail("missing field " + sym.Name)
	}
	recv.Obj.Fields[idx] = v
	return unit()
}

func (m *Machine) prefix(e *ast.PrefixExpr) Value {
	v := m.expr(e.X)
	if m.err != nil || m.ret {
		return m.retVal
	}
	sym := m.info.Syms[e]
	if sym != nil && sym.Builtin != types.BuiltinNone {
		return m.builtin(sym.Builtin, v, nil)
	}
	return m.callSym(sym, v, nil, nil)
}

func (m *Machine) infix(e *ast.InfixExpr) Value {
	if e.Op == "&&" {
		l := m.expr(e.Left)
		if m.err != nil || m.ret || !l.Truth() {
			if l.Tag == VBool {
				return l
			}
			return l
		}
		return m.expr(e.Right)
	}
	if e.Op == "||" {
		l := m.expr(e.Left)
		if m.err != nil || m.ret || l.Truth() {
			return l
		}
		return m.expr(e.Right)
	}
	recvExpr, argExpr := e.Left, e.Right
	if len(e.Op) > 0 && e.Op[len(e.Op)-1] == ':' {
		recvExpr, argExpr = e.Right, e.Left
	}
	recv := m.expr(recvExpr)
	if m.err != nil || m.ret {
		return m.retVal
	}
	arg := m.expr(argExpr)
	if m.err != nil || m.ret {
		return m.retVal
	}
	sym := m.info.Syms[e]
	return m.callSym(sym, recv, []Value{arg}, m.info.Evidence[e])
}

func nullaryMethod(sym *types.Symbol) bool {
	for _, list := range sym.Lists {
		if !list.Using && len(list.Params) > 0 {
			return false
		}
	}
	return true
}

func (m *Machine) selectExpr(e *ast.SelectExpr) Value {
	recv := m.expr(e.X)
	if m.err != nil || m.ret {
		return m.retVal
	}
	sym := m.info.Syms[e]
	if sym == nil {
		return m.fail("missing member " + e.Name)
	}
	switch sym.Kind {
	case types.SymTerm:
		if recv.Tag == VNull || recv.Obj == nil {
			return m.fail("null has no field " + e.Name)
		}
		idx, ok := m.index[sym]
		if !ok || idx >= len(recv.Obj.Fields) {
			return m.fail("missing field " + e.Name)
		}
		return recv.Obj.Fields[idx]
	case types.SymMethod:
		if nullaryMethod(sym) {
			return m.callSym(sym, recv, nil, m.info.Evidence[e])
		}
		return Value{Tag: VFun, Fun: &Fun{Sym: sym, Env: m.env, This: recv, HasThis: true}}
	case types.SymObject:
		return m.singleton(sym)
	case types.SymClass:
		return m.fail(sym.Name + " is a constructor")
	default:
		if recv.Obj != nil {
			if idx, ok := m.index[sym]; ok && idx < len(recv.Obj.Fields) {
				return recv.Obj.Fields[idx]
			}
		}
		return m.fail("bad selection " + e.Name)
	}
}

func (m *Machine) apply(e *ast.ApplyExpr) Value {
	if id, ok := e.Fun.(*ast.Ident); ok {
		sym := m.info.Syms[id]
		if sym != nil && (sym.Kind == types.SymClass || sym.Kind == types.SymTrait) {
			args := m.args(e.Args)
			if m.err != nil || m.ret {
				return m.retVal
			}
			return m.construct(sym, args)
		}
	}
	fun := m.expr(e.Fun)
	if m.err != nil || m.ret {
		return m.retVal
	}
	args := m.args(e.Args)
	if m.err != nil || m.ret {
		return m.retVal
	}
	return m.callVal(fun, args, m.info.Evidence[e])
}

func (m *Machine) args(es []ast.Expr) []Value {
	out := make([]Value, len(es))
	for i, e := range es {
		out[i] = m.expr(e)
		if m.err != nil || m.ret {
			return nil
		}
	}
	return out
}

func (m *Machine) lambda(e *ast.LambdaExpr) Value {
	var params []*types.Symbol
	for _, p := range e.Params {
		params = append(params, m.info.Syms[p])
	}
	return Value{Tag: VFun, Fun: &Fun{Lam: e, Env: m.env, Params: params}}
}

func (m *Machine) newObj(e *ast.NewExpr) Value {
	if len(e.Parents) == 0 {
		return m.fail("new without a class")
	}
	// The parent type was resolved on the parent node.
	pt := m.info.Types[e.Parents[0].Type]
	sym := classOf(pt)
	if sym == nil {
		return m.fail("new without a class")
	}
	args := m.args(e.Parents[0].Args)
	if m.err != nil || m.ret {
		return m.retVal
	}
	return m.construct(sym, args)
}

func (m *Machine) construct(class *types.Symbol, args []Value) Value {
	if class == nil {
		return m.fail("missing class")
	}
	m.layout(class)
	obj := &Object{Class: class, Fields: make([]Value, m.nfield[class])}
	for i, arg := range args {
		if i >= len(class.Ctor) {
			break
		}
		field := class.Ctor[i]
		if idx, ok := m.index[field]; ok && idx < len(obj.Fields) {
			obj.Fields[idx] = arg
		}
	}
	return Value{Tag: VObj, Obj: obj}
}

func (m *Machine) singleton(sym *types.Symbol) Value {
	if v, ok := m.single[sym]; ok {
		return v
	}
	m.layout(sym)
	obj := &Object{Class: sym, Fields: make([]Value, m.nfield[sym])}
	v := Value{Tag: VObj, Obj: obj}
	m.single[sym] = v
	return v
}

func (m *Machine) givenVal(sym *types.Symbol) Value {
	if v, ok := m.single[sym]; ok {
		return v
	}
	if g, ok := sym.AST.(*ast.GivenDecl); ok && g.Body != nil {
		v := m.expr(g.Body)
		m.single[sym] = v
		return v
	}
	obj := &Object{Class: sym}
	v := Value{Tag: VObj, Obj: obj}
	m.single[sym] = v
	return v
}

func (m *Machine) interp(e *ast.InterpExpr) Value {
	var b []byte
	for i, part := range e.Parts {
		b = append(b, part...)
		if i < len(e.Holes) {
			v := m.expr(e.Holes[i])
			if m.err != nil || m.ret {
				return m.retVal
			}
			b = append(b, v.String()...)
		}
	}
	return str(string(b))
}

func (m *Machine) callVal(fun Value, args []Value, evidence []*types.Symbol) Value {
	if fun.Tag != VFun || fun.Fun == nil {
		return m.fail("call of non-function")
	}
	f := fun.Fun
	if f.Sym != nil && f.Lam == nil {
		recv := Value{}
		if f.HasThis {
			recv = f.This
		}
		return m.callSym(f.Sym, recv, args, evidence)
	}
	if f.Lam == nil {
		return m.fail("empty function")
	}
	frame := newEnv(f.Env)
	for i, p := range f.Params {
		if p != nil && i < len(args) {
			frame.set(p, args[i])
		}
	}
	saved, savedThis := m.env, m.this
	savedRet, savedVal := m.ret, m.retVal
	m.env = frame
	m.ret = false
	v := m.expr(f.Lam.Body)
	if m.ret {
		v = m.retVal
	}
	m.ret = savedRet
	m.retVal = savedVal
	m.env = saved
	m.this = savedThis
	return v
}

func (m *Machine) resolveMethod(static *types.Symbol, recv Value) *types.Symbol {
	if static == nil || recv.Obj == nil || recv.Obj.Class == nil {
		return static
	}
	for _, s := range m.info.Ctx.Linearize(recv.Obj.Class) {
		impl := s.Get(static.Name)
		if impl == nil || impl.Kind != types.SymMethod {
			continue
		}
		if def, ok := impl.AST.(*ast.DefDecl); ok && def.Body != nil {
			return impl
		}
	}
	return static
}

func (m *Machine) callSym(sym *types.Symbol, recv Value, args []Value, evidence []*types.Symbol) Value {
	if sym == nil {
		return m.fail("missing method")
	}
	if m.depth > 100000 {
		return m.fail("stack overflow")
	}
	sym = m.resolveMethod(sym, recv)
	if sym.Builtin != types.BuiltinNone {
		return m.builtin(sym.Builtin, recv, args)
	}
	def, _ := sym.AST.(*ast.DefDecl)
	if def == nil || def.Body == nil {
		return m.fail(sym.Name + " has no body")
	}
	frame := newEnv(nil)
	if sym.Owner == nil || !sym.Owner.IsClassLike() {
		// Local and top-level functions close over the environment they were seen in.
		if v, ok := m.env.get(sym); ok && v.Fun != nil && v.Fun.Env != nil {
			frame.parent = v.Fun.Env
		}
	}
	ai, ei := 0, 0
	for _, list := range sym.Lists {
		if list.Using {
			for _, p := range list.Params {
				if ei < len(evidence) {
					frame.set(p, m.givenVal(evidence[ei]))
					ei++
				}
			}
			continue
		}
		for _, p := range list.Params {
			if ai < len(args) {
				frame.set(p, args[ai])
			}
			ai++
		}
	}
	saved, savedThis := m.env, m.this
	savedRet, savedVal := m.ret, m.retVal
	m.env = frame
	m.this = recv
	if recv.Tag == VObj && sym.Owner != nil {
		if th := sym.Owner.Get("this"); th != nil {
			frame.set(th, recv)
		}
	}
	if sym.Owner != nil && sym.Owner.Kind == types.SymTerm {
		frame.set(sym.Owner, recv)
	}
	m.ret = false
	m.stack = append(m.stack, sym.Name)
	m.depth++
	v := m.expr(def.Body)
	if m.ret {
		v = m.retVal
	}
	m.stack = m.stack[:len(m.stack)-1]
	m.depth--
	m.ret = savedRet
	m.retVal = savedVal
	m.env = saved
	m.this = savedThis
	return v
}
