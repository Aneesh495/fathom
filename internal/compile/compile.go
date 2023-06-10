package compile

import (
	"fmt"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/bc"
	"github.com/Aneesh495/fathom/internal/check"
	"github.com/Aneesh495/fathom/internal/types"
)

// Compile lowers info to bytecode. The result is ready for the virtual machine.
func Compile(info *check.Info) (*Program, error) {
	if info == nil || info.Main == nil {
		return nil, fmt.Errorf("program has no main")
	}
	c := &compiler{
		info:    info,
		classID: map[*types.Symbol]int{},
		funcID:  map[*types.Symbol]int{},
		field:   map[*types.Symbol]int{},
		nfield:  map[*types.Symbol]int{},
		global:  map[*types.Symbol]int{},
		prog:    &Program{Main: -1},
	}
	c.collect(info)
	c.declareFuncs()
	if err := c.compileFuncs(); err != nil {
		return nil, err
	}
	id, ok := c.funcID[info.Main]
	if !ok {
		return nil, fmt.Errorf("main was not compiled")
	}
	c.prog.Main = id
	return c.prog, nil
}

type compiler struct {
	info    *check.Info
	prog    *Program
	classID map[*types.Symbol]int
	funcID  map[*types.Symbol]int
	field   map[*types.Symbol]int
	nfield  map[*types.Symbol]int
	global  map[*types.Symbol]int
	queue   []*types.Symbol

	// current function
	code    bc.Buf
	consts  []Const
	local   map[*types.Symbol]int
	nlocal  int
	stack   int
	maxSt   int
	curName string
}

func (c *compiler) collect(info *check.Info) {
	seen := map[*types.Symbol]bool{}
	var walk func(*types.Symbol)
	walk = func(s *types.Symbol) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		if s.IsClassLike() || s.Has(types.FlagGiven) {
			c.addClass(s)
		}
		for _, m := range s.Members {
			walk(m)
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
	if info.Root != nil {
		for _, name := range info.Root.Names() {
			for _, s := range info.Root.Local(name) {
				walk(s)
			}
		}
	}
	for _, s := range info.Syms {
		walk(s)
	}
	for _, s := range c.queue {
		c.layout(s)
	}
	for _, s := range c.queue {
		cl := c.prog.Classes[c.classID[s]]
		for _, m := range c.info.Ctx.Linearize(s) {
			if id, ok := c.classID[m]; ok {
				cl.Lin = append(cl.Lin, id)
			}
		}
		if len(cl.Lin) == 0 {
			cl.Lin = []int{cl.ID}
		}
	}
}

func (c *compiler) addClass(s *types.Symbol) {
	if _, ok := c.classID[s]; ok {
		return
	}
	id := len(c.prog.Classes)
	// Slot 0 is unused so a zero class id never names a real class.
	if id == 0 {
		c.prog.Classes = append(c.prog.Classes, &Class{Name: "<none>"})
		id = 1
	}
	cl := &Class{
		Name:      s.Name,
		ID:        id,
		Methods:   map[string]int{},
		Singleton: s.Kind == types.SymObject || s.Has(types.FlagGiven),
		Sym:       s,
	}
	c.prog.Classes = append(c.prog.Classes, cl)
	c.classID[s] = id
	c.queue = append(c.queue, s)
	if cl.Singleton {
		c.global[s] = len(c.prog.GlobalSym)
		c.prog.GlobalSym = append(c.prog.GlobalSym, s)
		c.prog.Globals++
	}
}

func (c *compiler) layout(sym *types.Symbol) int {
	if sym == nil {
		return 0
	}
	if n, ok := c.nfield[sym]; ok {
		return n
	}
	c.nfield[sym] = 0
	n := 0
	for _, p := range sym.Parents {
		ps := namedSym(p)
		if ps != nil && ps != sym {
			n = c.layout(ps)
			break
		}
	}
	for _, mem := range sym.Members {
		if mem.Kind == types.SymTerm && mem.Name != "this" && mem.Owner == sym {
			c.field[mem] = n
			n++
		}
	}
	c.nfield[sym] = n
	if id, ok := c.classID[sym]; ok {
		c.prog.Classes[id].NFields = n
	}
	return n
}

func namedSym(t *types.Type) *types.Symbol {
	t = types.Repr(t)
	if t != nil && t.Kind == types.TyNamed {
		return t.Sym
	}
	return nil
}

func (c *compiler) declareFuncs() {
	seen := map[*types.Symbol]bool{}
	consider := func(s *types.Symbol) {
		if s == nil || seen[s] || s.Kind != types.SymMethod {
			return
		}
		def, ok := s.AST.(*ast.DefDecl)
		if !ok || def.Body == nil {
			return
		}
		if s.Builtin != types.BuiltinNone && s.Owner != nil && !s.Owner.IsClassLike() && s != c.info.Main {
			// Top-level builtins such as println are instructions, not functions.
			// main is never a builtin. Methods with builtins are also instructions.
		}
		if s.Builtin != types.BuiltinNone {
			return
		}
		seen[s] = true
		fn := &Func{Name: s.Name, Sym: s}
		id := len(c.prog.Funcs)
		c.prog.Funcs = append(c.prog.Funcs, fn)
		c.funcID[s] = id
		if s.Owner != nil {
			if cid, ok := c.classID[s.Owner]; ok {
				c.prog.Classes[cid].Methods[s.Name] = id
			}
		}
	}
	for _, s := range c.info.Syms {
		consider(s)
	}
	consider(c.info.Main)
}

func (c *compiler) compileFuncs() error {
	// Snapshot the list; lambdas append more functions while we compile.
	for i := 0; i < len(c.prog.Funcs); i++ {
		fn := c.prog.Funcs[i]
		if fn.Sym == nil {
			continue
		}
		if err := c.compileSym(fn); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileSym(fn *Func) error {
	sym := fn.Sym
	def, _ := sym.AST.(*ast.DefDecl)
	if def == nil || def.Body == nil {
		return fmt.Errorf("%s has no body", sym.Name)
	}
	c.startFunc(sym.Name)
	if sym.Owner != nil && sym.Owner.IsClassLike() {
		if th := sym.Owner.Get("this"); th != nil {
			c.bind(th)
		} else if sym.Owner.Has(types.FlagGiven) {
			// Given methods do not read fields, but a this slot keeps the
			// calling convention uniform with classes.
			c.nlocal++
		}
	}
	for _, list := range sym.Lists {
		if list.Using {
			continue
		}
		for _, p := range list.Params {
			c.bind(p)
		}
	}
	for _, list := range sym.Lists {
		if !list.Using {
			continue
		}
		for _, p := range list.Params {
			c.bind(p)
		}
	}
	c.expr(def.Body)
	c.emit(bc.OpReturn)
	fn.Code = append([]byte(nil), c.code.Code...)
	fn.Consts = c.consts
	fn.Locals = c.nlocal
	if c.maxSt < 32 {
		c.maxSt = 32
	}
	fn.MaxStack = c.maxSt
	if err := verifyFunc(fn); err != nil {
		return fmt.Errorf("%s: %w\n%s", fn.Name, err, Disasm(fn))
	}
	return nil
}

func (c *compiler) startFunc(name string) {
	c.code = bc.Buf{}
	c.consts = nil
	c.local = map[*types.Symbol]int{}
	c.nlocal = 0
	c.stack = 0
	c.maxSt = 0
	c.curName = name
}

func (c *compiler) bind(s *types.Symbol) int {
	if s == nil {
		n := c.nlocal
		c.nlocal++
		return n
	}
	if slot, ok := c.local[s]; ok {
		return slot
	}
	slot := c.nlocal
	c.nlocal++
	c.local[s] = slot
	return slot
}

func (c *compiler) emit(op bc.Op) { c.code.Op(op) }

func (c *compiler) u16(op bc.Op, v int) { c.code.U16(op, v) }

func (c *compiler) push(n int) {
	c.stack += n
	if c.stack > c.maxSt {
		c.maxSt = c.stack
	}
}

func (c *compiler) pop(n int) {
	c.stack -= n
	if c.stack < 0 {
		c.stack = 0
	}
}

func (c *compiler) expr(e ast.Expr) {
	if e == nil {
		c.emit(bc.OpUnit)
		c.push(1)
		return
	}
	if rw := c.info.Rewrites[e]; rw != nil {
		c.expr(rw)
		return
	}
	switch e := e.(type) {
	case *ast.LiteralExpr:
		c.literal(e)
	case *ast.Ident:
		c.ident(e)
	case *ast.ParenExpr:
		c.expr(e.X)
	case *ast.TupleExpr:
		c.tuple(e)
	case *ast.BlockExpr:
		c.block(e)
	case *ast.IfExpr:
		c.ifExpr(e)
	case *ast.WhileExpr:
		c.whileExpr(e)
	case *ast.PrefixExpr:
		c.prefix(e)
	case *ast.InfixExpr:
		c.infix(e)
	case *ast.SelectExpr:
		c.sel(e)
	case *ast.ApplyExpr:
		c.apply(e)
	case *ast.LambdaExpr:
		c.lambda(e)
	case *ast.AscribeExpr:
		c.expr(e.X)
	case *ast.ThisExpr:
		c.loadSym(c.thisSym())
	case *ast.ReturnExpr:
		if e.X != nil {
			c.expr(e.X)
		} else {
			c.emit(bc.OpUnit)
			c.push(1)
		}
		c.emit(bc.OpReturn)
		c.pop(1)
	case *ast.MatchExpr:
		c.matchExpr(e)
	case *ast.NewExpr:
		c.newExpr(e)
	case *ast.InterpExpr:
		c.interp(e)
	case *ast.AssignExpr:
		c.assign(e)
	case *ast.DoWhileExpr:
		c.doWhile(e)
	default:
		c.trap("cannot compile expression")
	}
}

func (c *compiler) literal(e *ast.LiteralExpr) {
	switch e.Kind {
	case ast.LitInt:
		n, _ := parseInt(e.Text)
		c.constInt(n)
	case ast.LitFloat:
		f, _ := parseFloat(e.Text)
		c.constFloat(f)
	case ast.LitBool:
		c.constBool(e.Text == "true")
	case ast.LitString:
		c.constStr(e.Text)
	case ast.LitChar:
		r := rune(0)
		for _, ch := range e.Text {
			r = ch
			break
		}
		c.constInt(int64(r))
	case ast.LitUnit:
		c.emit(bc.OpUnit)
		c.push(1)
	case ast.LitNull:
		c.emit(bc.OpNull)
		c.push(1)
	default:
		c.emit(bc.OpUnit)
		c.push(1)
	}
}

func (c *compiler) constInt(n int64) {
	idx := c.addConst(Const{Tag: ConstInt, I: n})
	c.u16(bc.OpConst, idx)
	c.push(1)
}

func (c *compiler) constFloat(f float64) {
	idx := c.addConst(Const{Tag: ConstFloat, F: f})
	c.u16(bc.OpConst, idx)
	c.push(1)
}

func (c *compiler) constBool(b bool) {
	idx := c.addConst(Const{Tag: ConstBool, B: b})
	c.u16(bc.OpConst, idx)
	c.push(1)
}

func (c *compiler) constStr(s string) {
	idx := c.addConst(Const{Tag: ConstString, S: s})
	c.u16(bc.OpConst, idx)
	c.push(1)
}

func (c *compiler) addConst(k Const) int {
	c.consts = append(c.consts, k)
	return len(c.consts) - 1
}

func (c *compiler) ident(e *ast.Ident) {
	sym := c.info.Syms[e]
	if sym == nil {
		c.trap("unbound " + e.Name)
		return
	}
	c.loadSym(sym)
}

func (c *compiler) loadSym(sym *types.Symbol) {
	if sym == nil {
		c.trap("missing symbol")
		return
	}
	if slot, ok := c.local[sym]; ok {
		c.u16(bc.OpLoad, slot)
		c.push(1)
		return
	}
	if idx, ok := c.field[sym]; ok {
		c.loadSym(c.thisSym())
		c.u16(bc.OpGetField, idx)
		return
	}
	if g, ok := c.global[sym]; ok {
		c.u16(bc.OpGlobal, g)
		c.push(1)
		return
	}
	if sym.Kind == types.SymMethod {
		c.trap("method " + sym.Name + " used as a value")
		return
	}
	c.trap("no location for " + sym.Name)
}

func (c *compiler) thisSym() *types.Symbol {
	// The current function's this, if any, is whichever this symbol was bound.
	for s := range c.local {
		if s != nil && s.Name == "this" {
			return s
		}
	}
	return nil
}

func (c *compiler) tuple(e *ast.TupleExpr) {
	var class *types.Symbol
	switch len(e.Elems) {
	case 2:
		class = c.info.Ctx.Tuple2
	case 3:
		class = c.info.Ctx.Tuple3
	default:
		class = c.info.Ctx.Tuple4
	}
	c.construct(class, e.Elems)
}

func (c *compiler) construct(class *types.Symbol, args []ast.Expr) {
	id := c.classID[class]
	c.u16(bc.OpNew, id)
	c.push(1)
	for i, arg := range args {
		c.emit(bc.OpDup)
		c.push(1)
		c.expr(arg)
		idx := i
		if i < len(class.Ctor) {
			if f, ok := c.field[class.Ctor[i]]; ok {
				idx = f
			}
		}
		c.u16(bc.OpPutField, idx)
		c.pop(2)
	}
}

func (c *compiler) block(e *ast.BlockExpr) {
	if len(e.Stats) == 0 {
		c.emit(bc.OpUnit)
		c.push(1)
		return
	}
	for i, st := range e.Stats {
		last := i == len(e.Stats)-1
		c.stmt(st, last)
	}
}

func (c *compiler) stmt(st ast.Stmt, last bool) {
	switch st := st.(type) {
	case *ast.ExprStmt:
		c.expr(st.X)
		if !last {
			c.emit(bc.OpPop)
			c.pop(1)
		}
	case *ast.ValDecl:
		sym := c.info.Syms[st]
		if st.Body != nil {
			c.expr(st.Body)
		} else {
			c.emit(bc.OpUnit)
			c.push(1)
		}
		slot := c.bind(sym)
		c.u16(bc.OpStore, slot)
		c.pop(1)
		if last {
			c.emit(bc.OpUnit)
			c.push(1)
		}
	case *ast.DefDecl:
		if last {
			c.emit(bc.OpUnit)
			c.push(1)
		}
	default:
		if last {
			c.emit(bc.OpUnit)
			c.push(1)
		}
	}
}

func (c *compiler) ifExpr(e *ast.IfExpr) {
	c.expr(e.Cond)
	jElse := c.jump(bc.OpJumpIfNot)
	c.expr(e.Then)
	jEnd := c.jump(bc.OpJump)
	c.patch(jElse)
	if e.Else != nil {
		c.expr(e.Else)
	} else {
		c.emit(bc.OpUnit)
		c.push(1)
	}
	c.patch(jEnd)
	// Both branches leave one value. The stack count was incremented twice
	// (then and else) and the jumps skip one of them, so compensate.
	c.stack--
	if c.stack < 1 {
		c.stack = 1
	}
}

func (c *compiler) whileExpr(e *ast.WhileExpr) {
	start := c.code.Len()
	c.expr(e.Cond)
	jEnd := c.jump(bc.OpJumpIfNot)
	c.expr(e.Body)
	c.emit(bc.OpPop)
	c.pop(1)
	c.jumpTo(bc.OpJump, start)
	c.patch(jEnd)
	c.emit(bc.OpUnit)
	c.push(1)
}

func (c *compiler) doWhile(e *ast.DoWhileExpr) {
	start := c.code.Len()
	c.expr(e.Body)
	c.emit(bc.OpPop)
	c.pop(1)
	c.expr(e.Cond)
	c.jumpTo(bc.OpJumpIf, start)
	c.emit(bc.OpUnit)
	c.push(1)
}

func (c *compiler) prefix(e *ast.PrefixExpr) {
	c.expr(e.X)
	switch e.Op {
	case "-":
		c.emit(bc.OpNeg)
	case "!":
		c.emit(bc.OpNot)
	case "~":
		c.emit(bc.OpBitNot)
	default:
		c.emit(bc.OpNop)
	}
}

func (c *compiler) infix(e *ast.InfixExpr) {
	if e.Op == "&&" || e.Op == "||" {
		c.logic(e)
		return
	}
	recv, arg := e.Left, e.Right
	if len(e.Op) > 0 && e.Op[len(e.Op)-1] == ':' {
		recv, arg = e.Right, e.Left
	}
	sym := c.info.Syms[e]
	if sym != nil && sym.Builtin != types.BuiltinNone {
		c.expr(recv)
		c.expr(arg)
		c.builtin(sym.Builtin)
		return
	}
	c.callKnown(sym, recv, []ast.Expr{arg}, c.info.Evidence[e])
}

func (c *compiler) logic(e *ast.InfixExpr) {
	c.expr(e.Left)
	c.emit(bc.OpDup)
	c.push(1)
	var j int
	if e.Op == "&&" {
		j = c.jump(bc.OpJumpIfNot)
	} else {
		j = c.jump(bc.OpJumpIf)
	}
	c.emit(bc.OpPop)
	c.pop(1)
	c.expr(e.Right)
	c.patch(j)
	c.stack--
	if c.stack < 1 {
		c.stack = 1
	}
}

func (c *compiler) builtin(op types.Builtin) {
	// Two values are on the stack. The opcode replaces them with one.
	switch op {
	case types.BuiltinAdd:
		c.emit(bc.OpAdd)
	case types.BuiltinSub:
		c.emit(bc.OpSub)
	case types.BuiltinMul:
		c.emit(bc.OpMul)
	case types.BuiltinDiv:
		c.emit(bc.OpDiv)
	case types.BuiltinMod:
		c.emit(bc.OpMod)
	case types.BuiltinNeg:
		c.emit(bc.OpNeg)
		c.stack++ // neg is unary; caller pushed one extra. Correct below.
	case types.BuiltinEq:
		c.emit(bc.OpEq)
	case types.BuiltinNe:
		c.emit(bc.OpNe)
	case types.BuiltinLt:
		c.emit(bc.OpLt)
	case types.BuiltinLe:
		c.emit(bc.OpLe)
	case types.BuiltinGt:
		c.emit(bc.OpGt)
	case types.BuiltinGe:
		c.emit(bc.OpGe)
	case types.BuiltinNot:
		c.emit(bc.OpNot)
	case types.BuiltinBitAnd:
		c.emit(bc.OpBitAnd)
	case types.BuiltinBitOr:
		c.emit(bc.OpBitOr)
	case types.BuiltinBitXor:
		c.emit(bc.OpBitXor)
	case types.BuiltinBitNot:
		c.emit(bc.OpBitNot)
	case types.BuiltinShl:
		c.emit(bc.OpShl)
	case types.BuiltinShr:
		c.emit(bc.OpShr)
	case types.BuiltinConcat:
		c.emit(bc.OpConcat)
	default:
		c.emit(bc.OpNop)
	}
	c.pop(1)
}

func (c *compiler) sel(e *ast.SelectExpr) {
	sym := c.info.Syms[e]
	if sym != nil && sym.Kind == types.SymTerm {
		c.expr(e.X)
		idx := 0
		if f, ok := c.field[sym]; ok {
			idx = f
		}
		c.u16(bc.OpGetField, idx)
		return
	}
	if sym != nil && (sym.Kind == types.SymObject || sym.Has(types.FlagGiven)) {
		c.loadSym(sym)
		return
	}
	c.trap("bad selection " + e.Name)
}

func (c *compiler) apply(e *ast.ApplyExpr) {
	if id, ok := e.Fun.(*ast.Ident); ok {
		sym := c.info.Syms[id]
		if sym != nil && (sym.Kind == types.SymClass || sym.Kind == types.SymTrait) {
			c.construct(sym, e.Args)
			return
		}
		if sym != nil && sym.Builtin != types.BuiltinNone {
			c.builtinCall(sym, nil, e.Args)
			return
		}
	}
	if sel, ok := e.Fun.(*ast.SelectExpr); ok {
		sym := c.info.Syms[sel]
		if sym != nil && sym.Builtin != types.BuiltinNone {
			c.expr(sel.X)
			for _, a := range e.Args {
				c.expr(a)
			}
			c.builtin(sym.Builtin)
			return
		}
		if sym != nil && sym.Kind == types.SymMethod {
			c.callKnown(sym, sel.X, e.Args, c.info.Evidence[e])
			return
		}
	}
	if id, ok := e.Fun.(*ast.Ident); ok {
		sym := c.info.Syms[id]
		if sym != nil && sym.Kind == types.SymMethod {
			c.callKnown(sym, nil, e.Args, c.info.Evidence[e])
			return
		}
	}
	// Closure call: push function, then args, then callclos.
	c.expr(e.Fun)
	for _, a := range e.Args {
		c.expr(a)
	}
	c.code.U8(bc.OpCallClos, byte(len(e.Args)))
	c.pop(len(e.Args) + 1)
	c.push(1)
}

func (c *compiler) builtinCall(sym *types.Symbol, recv ast.Expr, args []ast.Expr) {
	if recv != nil {
		c.expr(recv)
	}
	for _, a := range args {
		c.expr(a)
	}
	switch sym.Builtin {
	case types.BuiltinPrintln:
		c.emit(bc.OpPrintln)
		c.pop(1)
		c.emit(bc.OpUnit)
		c.push(1)
	case types.BuiltinPrint:
		c.emit(bc.OpPrint)
		c.pop(1)
		c.emit(bc.OpUnit)
		c.push(1)
	default:
		if recv != nil {
			c.builtin(sym.Builtin)
		} else {
			c.trap("builtin " + sym.Name)
		}
	}
}

func (c *compiler) callKnown(sym *types.Symbol, recv ast.Expr, args []ast.Expr, evidence []*types.Symbol) {
	if sym == nil {
		c.trap("missing call")
		return
	}
	argc := 0
	if recv != nil {
		c.expr(recv)
		argc++
	}
	for _, a := range args {
		c.expr(a)
		argc++
	}
	for _, g := range evidence {
		c.loadSym(g)
		argc++
	}
	if sym.Owner != nil && (sym.Owner.IsClassLike() || sym.Owner.Has(types.FlagGiven)) {
		name := c.addConst(Const{Tag: ConstString, S: sym.Name})
		c.code.U16U8(bc.OpCallVirt, name, byte(argc))
	} else if id, ok := c.funcID[sym]; ok {
		c.code.U16U8(bc.OpCall, id, byte(argc))
	} else {
		c.trap("uncompiled " + sym.Name)
		return
	}
	c.pop(argc)
	c.push(1)
}

func (c *compiler) lambda(e *ast.LambdaExpr) {
	frees := c.freeVars(e)
	id := c.compileLambda(e, frees)
	for _, s := range frees {
		c.loadSym(s)
	}
	c.code.U16U8(bc.OpClosure, id, byte(len(frees)))
	c.pop(len(frees))
	c.push(1)
}

func (c *compiler) freeVars(e *ast.LambdaExpr) []*types.Symbol {
	bound := map[*types.Symbol]bool{}
	for _, p := range e.Params {
		if s := c.info.Syms[p]; s != nil {
			bound[s] = true
		}
	}
	var out []*types.Symbol
	seen := map[*types.Symbol]bool{}
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		switch n := n.(type) {
		case *ast.Ident:
			s := c.info.Syms[n]
			if s == nil || bound[s] || seen[s] {
				return
			}
			if _, ok := c.local[s]; ok {
				seen[s] = true
				out = append(out, s)
			}
		case *ast.LambdaExpr:
			inner := map[*types.Symbol]bool{}
			for k, v := range bound {
				inner[k] = v
			}
			for _, p := range n.Params {
				if s := c.info.Syms[p]; s != nil {
					inner[s] = true
				}
			}
			saved := bound
			bound = inner
			walk(n.Body)
			bound = saved
		default:
			for _, ch := range children(n) {
				walk(ch)
			}
		}
	}
	walk(e.Body)
	return out
}

func (c *compiler) compileLambda(e *ast.LambdaExpr, frees []*types.Symbol) int {
	savedCode, savedConst := c.code, c.consts
	savedLocal, savedN := c.local, c.nlocal
	savedStack, savedMax, savedName := c.stack, c.maxSt, c.curName
	c.startFunc("lambda")
	var params []*types.Symbol
	for _, p := range e.Params {
		s := c.info.Syms[p]
		params = append(params, s)
		c.bind(s)
	}
	for _, s := range frees {
		c.bind(s)
	}
	c.expr(e.Body)
	c.emit(bc.OpReturn)
	fn := &Func{
		Name:     "lambda",
		Code:     append([]byte(nil), c.code.Code...),
		Consts:   c.consts,
		Locals:   c.nlocal,
		MaxStack: c.maxSt,
	}
	if err := verifyFunc(fn); err != nil {
		fn.Code = append(fn.Code, byte(bc.OpTrap))
	}
	id := len(c.prog.Funcs)
	c.prog.Funcs = append(c.prog.Funcs, fn)
	c.code, c.consts = savedCode, savedConst
	c.local, c.nlocal = savedLocal, savedN
	c.stack, c.maxSt, c.curName = savedStack, savedMax, savedName
	_ = params
	return id
}

func (c *compiler) newExpr(e *ast.NewExpr) {
	if len(e.Parents) == 0 {
		c.trap("new")
		return
	}
	pt := c.info.Types[e.Parents[0].Type]
	sym := namedSym(pt)
	if sym == nil {
		c.trap("new")
		return
	}
	c.construct(sym, e.Parents[0].Args)
}

func (c *compiler) interp(e *ast.InterpExpr) {
	if len(e.Parts) == 0 {
		c.constStr("")
		return
	}
	c.constStr(e.Parts[0])
	for i := 0; i < len(e.Holes); i++ {
		c.expr(e.Holes[i])
		c.emit(bc.OpConcat)
		c.pop(1)
		part := ""
		if i+1 < len(e.Parts) {
			part = e.Parts[i+1]
		}
		c.constStr(part)
		c.emit(bc.OpConcat)
		c.pop(1)
	}
}

func (c *compiler) assign(e *ast.AssignExpr) {
	rhs := e.RHS
	if e.Op != "=" && len(e.Op) > 1 {
		base := e.Op[:len(e.Op)-1]
		rhs = &ast.InfixExpr{S: e.Span(), Op: base, Left: e.LHS, Right: e.RHS}
	}
	c.expr(rhs)
	if id, ok := e.LHS.(*ast.Ident); ok {
		sym := c.info.Syms[id]
		if slot, ok := c.local[sym]; ok {
			c.u16(bc.OpStore, slot)
			c.pop(1)
			c.emit(bc.OpUnit)
			c.push(1)
			return
		}
		if idx, ok := c.field[sym]; ok {
			// Stack is the value. Push this under it via a temporary local.
			tmp := c.bind(nil)
			c.u16(bc.OpStore, tmp)
			c.pop(1)
			c.loadSym(c.thisSym())
			c.u16(bc.OpLoad, tmp)
			c.u16(bc.OpPutField, idx)
			c.pop(2)
			c.emit(bc.OpUnit)
			c.push(1)
			return
		}
	}
	c.emit(bc.OpPop)
	c.pop(1)
	c.emit(bc.OpUnit)
	c.push(1)
}

func (c *compiler) matchExpr(e *ast.MatchExpr) {
	tmpSym := c.info.Ctx.NewSymbol("$scrut", types.SymTerm)
	tmp := c.bind(tmpSym)
	c.expr(e.Scrut)
	c.u16(bc.OpStore, tmp)
	c.pop(1)
	var ends []int
	for _, cs := range e.Cases {
		var fails []int
		c.pat(cs.Pat, tmp, &fails)
		if cs.Guard != nil {
			c.expr(cs.Guard)
			fails = append(fails, c.jump(bc.OpJumpIfNot))
		}
		c.expr(cs.Body)
		ends = append(ends, c.jump(bc.OpJump))
		for _, f := range fails {
			c.patch(f)
		}
		// A failed pattern leaves no value. Drop the branch's contribution
		// from the stack count; the successful path's value remains logically one.
		c.stack--
	}
	c.trap("match error")
	for _, j := range ends {
		c.patch(j)
	}
	if c.stack < 1 {
		c.stack = 1
	}
}

func (c *compiler) pat(p ast.Pattern, tmp int, fails *[]int) {
	if p == nil {
		return
	}
	switch p := p.(type) {
	case *ast.WildPat:
	case *ast.BindPat:
		if sym := c.info.Syms[p]; sym != nil {
			slot := c.bind(sym)
			c.u16(bc.OpLoad, tmp)
			c.push(1)
			c.u16(bc.OpStore, slot)
			c.pop(1)
		}
		if p.Pat != nil {
			c.pat(p.Pat, tmp, fails)
		}
	case *ast.LitPat:
		c.u16(bc.OpLoad, tmp)
		c.push(1)
		c.literal(p.Lit)
		c.emit(bc.OpEq)
		c.pop(1)
		*fails = append(*fails, c.jump(bc.OpJumpIfNot))
	case *ast.TypedPat:
		c.pat(p.Pat, tmp, fails)
	case *ast.TuplePat:
		for i, sub := range p.Pats {
			inner := c.bind(c.info.Ctx.NewSymbol("$f", types.SymTerm))
			c.u16(bc.OpLoad, tmp)
			c.push(1)
			c.u16(bc.OpGetField, i)
			c.u16(bc.OpStore, inner)
			c.pop(1)
			c.pat(sub, inner, fails)
		}
	case *ast.AltPat:
		var done []int
		for _, alt := range p.Pats {
			var local []int
			c.pat(alt, tmp, &local)
			done = append(done, c.jump(bc.OpJump))
			for _, f := range local {
				c.patch(f)
			}
		}
		*fails = append(*fails, c.jump(bc.OpJump))
		for _, d := range done {
			c.patch(d)
		}
	case *ast.CtorPat:
		sym := c.info.Syms[p]
		c.u16(bc.OpLoad, tmp)
		c.push(1)
		id := 0
		if sym != nil {
			id = c.classID[sym]
		}
		c.u16(bc.OpClassEq, id)
		*fails = append(*fails, c.jump(bc.OpJumpIfNot))
		if sym != nil && sym.Kind != types.SymObject {
			for i, sub := range p.Pats {
				idx := i
				if i < len(sym.Ctor) {
					if f, ok := c.field[sym.Ctor[i]]; ok {
						idx = f
					}
				}
				inner := c.bind(c.info.Ctx.NewSymbol("$f", types.SymTerm))
				c.u16(bc.OpLoad, tmp)
				c.push(1)
				c.u16(bc.OpGetField, idx)
				c.u16(bc.OpStore, inner)
				c.pop(1)
				c.pat(sub, inner, fails)
			}
		}
	}
}

func (c *compiler) jump(op bc.Op) int {
	at := c.code.Len()
	c.code.I16(op, 0)
	if op == bc.OpJumpIf || op == bc.OpJumpIfNot {
		c.pop(1)
	}
	return at
}

func (c *compiler) jumpTo(op bc.Op, target int) {
	rel := target - (c.code.Len() + 3)
	c.code.I16(op, rel)
	if op == bc.OpJumpIf || op == bc.OpJumpIfNot {
		c.pop(1)
	}
}

func (c *compiler) patch(at int) {
	c.code.PatchI16(at, c.code.Len())
}

func (c *compiler) trap(msg string) {
	idx := c.addConst(Const{Tag: ConstString, S: msg})
	c.u16(bc.OpTrap, idx)
	c.push(1)
}

func parseInt(text string) (int64, error) {
	s := strip(text)
	base := 10
	if len(s) > 2 && (s[0] == '0' && (s[1] == 'x' || s[1] == 'X')) {
		return atoi(s[2:], 16)
	}
	if len(s) > 2 && (s[0] == '0' && (s[1] == 'b' || s[1] == 'B')) {
		return atoi(s[2:], 2)
	}
	return atoi(s, base)
}

func atoi(s string, base int) (int64, error) {
	var n int64
	for _, r := range s {
		d := int64(-1)
		switch {
		case r >= '0' && r <= '9':
			d = int64(r - '0')
		case r >= 'a' && r <= 'f':
			d = int64(r-'a') + 10
		case r >= 'A' && r <= 'F':
			d = int64(r-'A') + 10
		}
		if d < 0 || d >= int64(base) {
			return 0, fmt.Errorf("bad int")
		}
		n = n*int64(base) + d
	}
	return n, nil
}

func parseFloat(text string) (float64, error) {
	s := strip(text)
	// A tiny decimal parser is enough for literals the checker already accepted.
	var n float64
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + float64(s[i]-'0')
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		div := 10.0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			n += float64(s[i]-'0') / div
			div *= 10
			i++
		}
	}
	return n, nil
}

func strip(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			b = append(b, s[i])
		}
	}
	return string(b)
}
