package check

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
	"github.com/Aneesh495/fathom/internal/types"
)

func (ck *Checker) synth(e ast.Expr) *types.Type {
	if e == nil {
		return ck.Ctx.Error
	}
	if t, ok := ck.types[e]; ok && t != nil && e != nil {
		// Bodies are checked once. A second lookup during errors returns the note.
		// Lambdas and blocks are stored after checking, so this is safe.
	}
	t := ck.synth1(e)
	return ck.noteType(e, t)
}

func (ck *Checker) synth1(e ast.Expr) *types.Type {
	switch e := e.(type) {
	case *ast.LiteralExpr:
		return ck.litType(e)
	case *ast.Ident:
		return ck.synthIdent(e)
	case *ast.ParenExpr:
		return ck.synth(e.X)
	case *ast.TupleExpr:
		return ck.synthTuple(e)
	case *ast.BlockExpr:
		return ck.checkBlock(e, nil)
	case *ast.IfExpr:
		return ck.synthIf(e, nil)
	case *ast.WhileExpr:
		ck.check(e.Cond, ck.Ctx.BooleanType())
		ck.synth(e.Body)
		return ck.Ctx.UnitType()
	case *ast.DoWhileExpr:
		ck.synth(e.Body)
		ck.check(e.Cond, ck.Ctx.BooleanType())
		return ck.Ctx.UnitType()
	case *ast.AssignExpr:
		return ck.synthAssign(e)
	case *ast.PrefixExpr:
		return ck.synthPrefix(e)
	case *ast.InfixExpr:
		return ck.synthInfix(e)
	case *ast.SelectExpr:
		return ck.synthSelect(e)
	case *ast.ApplyExpr:
		return ck.synthApply(e)
	case *ast.TypeApplyExpr:
		return ck.synthTypeApply(e)
	case *ast.LambdaExpr:
		return ck.checkLambda(e, nil)
	case *ast.AscribeExpr:
		want := ck.resolveType(e.Type)
		return ck.check(e.X, want)
	case *ast.ThisExpr:
		if ck.this == nil {
			ck.errorf(e.Span(), "this is not in scope")
			return ck.Ctx.Error
		}
		ck.noteSym(e, ck.this)
		return ck.this.Type
	case *ast.SuperExpr:
		return ck.synthSuper(e)
	case *ast.ReturnExpr:
		if ck.ret == nil {
			if e.X != nil {
				ck.synth(e.X)
			}
		} else if e.X == nil {
			if !ck.Ctx.Subtype(ck.Ctx.UnitType(), ck.ret) {
				ck.mismatch(e.Span(), ck.Ctx.UnitType(), ck.ret)
			}
		} else {
			ck.check(e.X, ck.ret)
		}
		return ck.Ctx.Bot
	case *ast.ThrowExpr:
		ck.synth(e.X)
		return ck.Ctx.Bot
	case *ast.MatchExpr:
		return ck.synthMatch(e, nil)
	case *ast.ForExpr:
		return ck.synthFor(e)
	case *ast.NewExpr:
		return ck.synthNew(e)
	case *ast.InterpExpr:
		return ck.synthInterp(e)
	case *ast.TryExpr:
		t := ck.synth(e.Body)
		for _, c := range e.Catches {
			ck.synthCase(t, c, nil)
		}
		if e.Finally != nil {
			ck.synth(e.Finally)
		}
		return t
	case *ast.WildcardExpr:
		ck.errorf(e.Span(), "wildcard is not an expression")
		return ck.Ctx.Error
	default:
		ck.errorf(e.Span(), "unsupported expression")
		return ck.Ctx.Error
	}
}

func (ck *Checker) check(e ast.Expr, want *types.Type) *types.Type {
	if e == nil {
		return ck.Ctx.Error
	}
	want = types.Repr(want)
	switch e := e.(type) {
	case *ast.LambdaExpr:
		return ck.noteType(e, ck.checkLambda(e, want))
	case *ast.IfExpr:
		return ck.noteType(e, ck.synthIf(e, want))
	case *ast.BlockExpr:
		return ck.noteType(e, ck.checkBlock(e, want))
	case *ast.MatchExpr:
		return ck.noteType(e, ck.synthMatch(e, want))
	}
	got := ck.synth(e)
	if want != nil && !want.IsError() && !got.IsError() && !ck.Ctx.Subtype(got, want) {
		if !ck.Ctx.Constrain(got, want) {
			ck.mismatch(e.Span(), got, want)
		}
	}
	return got
}

func (ck *Checker) synthIdent(e *ast.Ident) *types.Type {
	if e.Name == "_" {
		ck.errorf(e.Span(), "wildcard is not an expression")
		return ck.Ctx.Error
	}
	hits := ck.lookup(e.Name)
	sym := preferTerm(hits)
	if sym == nil {
		sym = preferType(hits)
	}
	if sym == nil {
		ck.errorf(e.Span(), "not found: %s", e.Name)
		return ck.Ctx.Error
	}
	ck.noteSym(e, sym)
	switch sym.Kind {
	case types.SymTerm, types.SymObject:
		if sym.Type == nil {
			ck.errorf(e.Span(), "%s is used before its type is known", e.Name)
			return ck.Ctx.Error
		}
		return sym.Type
	case types.SymMethod:
		if sym.Type == nil && sym.Result != nil {
			ck.finishMethodType(sym)
		}
		if sym.Type == nil {
			ck.errorf(e.Span(), "%s is used before its type is known", e.Name)
			return ck.Ctx.Error
		}
		return ck.openReceiver(sym, ck.receiverType(sym))
	case types.SymClass, types.SymTrait:
		return ck.Ctx.Named(sym, nil)
	default:
		ck.errorf(e.Span(), "%s cannot be used as a value", e.Name)
		return ck.Ctx.Error
	}
}

func (ck *Checker) receiverType(method *types.Symbol) *types.Type {
	if method == nil || method.Owner == nil || !method.Owner.IsClassLike() {
		return nil
	}
	if ck.this != nil && ck.this.Owner == method.Owner {
		return ck.this.Type
	}
	return method.Owner.Type
}

func (ck *Checker) openReceiver(method *types.Symbol, recv *types.Type) *types.Type {
	if method == nil {
		return ck.Ctx.Error
	}
	base := method.Type
	if base == nil {
		base = method.Result
	}
	if recv == nil {
		return base
	}
	return substSymType(ck.Ctx, base, ck.inheritSubst(recv, method.Owner))
}

func (ck *Checker) synthTuple(e *ast.TupleExpr) *types.Type {
	args := make([]ast.Expr, len(e.Elems))
	copy(args, e.Elems)
	var class *types.Symbol
	switch len(args) {
	case 2:
		class = ck.Ctx.Tuple2
	case 3:
		class = ck.Ctx.Tuple3
	case 4:
		class = ck.Ctx.Tuple4
	default:
		ck.errorf(e.Span(), "tuples have 2, 3, or 4 elements")
		return ck.Ctx.Error
	}
	return ck.applyCtor(e.Span(), class, nil, args)
}

func (ck *Checker) checkBlock(e *ast.BlockExpr, want *types.Type) *types.Type {
	prev := ck.scope
	ck.scope = types.NewScope(prev, prev.Owner)
	defer func() { ck.scope = prev }()
	if len(e.Stats) == 0 {
		if want != nil && !ck.Ctx.Subtype(ck.Ctx.UnitType(), want) && !want.IsError() {
			ck.mismatch(e.Span(), ck.Ctx.UnitType(), want)
		}
		return ck.Ctx.UnitType()
	}
	for i, st := range e.Stats {
		last := i == len(e.Stats)-1
		if es, ok := st.(*ast.ExprStmt); ok && last {
			if want != nil {
				return ck.check(es.X, want)
			}
			return ck.synth(es.X)
		}
		if last {
			if vd, ok := st.(*ast.ValDecl); ok {
				ck.enterStmt(vd, ck.scope)
				ck.sigStmt(vd)
				ck.checkStmt(vd)
			} else if dd, ok := st.(*ast.DefDecl); ok {
				ck.enterStmt(dd, ck.scope)
				ck.sigStmt(dd)
				ck.checkStmt(dd)
			} else {
				ck.checkStmt(st)
			}
			return ck.Ctx.UnitType()
		}
		switch s := st.(type) {
		case *ast.ValDecl:
			ck.enterStmt(s, ck.scope)
			ck.sigStmt(s)
			ck.checkStmt(s)
		case *ast.DefDecl:
			ck.enterStmt(s, ck.scope)
			ck.sigStmt(s)
			ck.checkStmt(s)
		case *ast.ClassDecl:
			ck.enterStmt(s, ck.scope)
			ck.sigStmt(s)
			ck.checkStmt(s)
		default:
			ck.checkStmt(s)
		}
	}
	return ck.Ctx.UnitType()
}

func (ck *Checker) synthIf(e *ast.IfExpr, want *types.Type) *types.Type {
	ck.check(e.Cond, ck.Ctx.BooleanType())
	if e.Else == nil {
		ck.synth(e.Then)
		return ck.Ctx.UnitType()
	}
	if want != nil && !want.IsError() {
		ck.check(e.Then, want)
		ck.check(e.Else, want)
		return want
	}
	a := ck.synth(e.Then)
	b := ck.synth(e.Else)
	return ck.Ctx.Lub(a, b)
}

func (ck *Checker) synthAssign(e *ast.AssignExpr) *types.Type {
	if e.Op != "=" {
		// x += y  is  x = x + y, with the operator stripped of '='.
		base := token.AssignBase(e.Op)
		left := ck.synth(e.LHS)
		ck.noteType(e.LHS, left)
		rhs := &ast.InfixExpr{S: e.Span(), Op: base, Left: e.LHS, Right: e.RHS}
		ck.check(rhs, left)
		ck.requireMutable(e.LHS)
		return ck.Ctx.UnitType()
	}
	left := ck.synth(e.LHS)
	ck.check(e.RHS, left)
	ck.requireMutable(e.LHS)
	return ck.Ctx.UnitType()
}

func (ck *Checker) requireMutable(e ast.Expr) {
	switch e := e.(type) {
	case *ast.Ident:
		sym := ck.syms[e]
		if sym == nil || !sym.Has(types.FlagMutable) {
			ck.errorf(e.Span(), "cannot assign to %s", e.Name)
		}
	case *ast.SelectExpr:
		sym := ck.syms[e]
		if sym == nil || !sym.Has(types.FlagMutable) {
			ck.errorf(e.Span(), "cannot assign to %s", e.Name)
		}
	default:
		ck.errorf(e.Span(), "assignment target must be a variable")
	}
}

func (ck *Checker) synthPrefix(e *ast.PrefixExpr) *types.Type {
	name := "unary_" + e.Op
	recv := ck.synth(e.X)
	m := ck.Ctx.LookupMember(recv, name)
	if m == nil {
		ck.errorf(e.Span(), "%s has no method %s", ck.Ctx.Format(recv), name)
		return ck.Ctx.Error
	}
	ck.noteSym(e, m)
	return ck.openReceiver(m, recv)
}

func (ck *Checker) synthInfix(e *ast.InfixExpr) *types.Type {
	if e.Op == "&&" || e.Op == "||" {
		ck.check(e.Left, ck.Ctx.BooleanType())
		ck.check(e.Right, ck.Ctx.BooleanType())
		return ck.Ctx.BooleanType()
	}
	recvExpr := e.Left
	arg := e.Right
	name := e.Op
	if len(name) > 0 && name[len(name)-1] == ':' {
		recvExpr = e.Right
		arg = e.Left
	}
	recv := ck.synth(recvExpr)
	m := ck.lookupMemberOrExt(recv, name, recvExpr)
	if m == nil {
		ck.errorf(e.Span(), "%s has no method %s", ck.Ctx.Format(recv), name)
		return ck.Ctx.Error
	}
	ck.noteSym(e, m)
	prev := ck.subject
	ck.subject = e
	defer func() { ck.subject = prev }()
	return ck.applySymbol(e.Span(), m, recv, nil, []ast.Expr{arg}, false)
}

func (ck *Checker) synthSelect(e *ast.SelectExpr) *types.Type {
	recv := ck.synth(e.X)
	if recv.IsError() {
		return recv
	}
	m := ck.lookupMemberOrExt(recv, e.Name, e.X)
	if m == nil {
		ck.errorf(e.Span(), "%s is not a member of %s", e.Name, ck.Ctx.Format(recv))
		return ck.Ctx.Error
	}
	ck.noteSym(e, m)
	switch m.Kind {
	case types.SymTerm:
		return substSymType(ck.Ctx, m.Type, ck.inheritSubst(recv, m.Owner))
	case types.SymMethod:
		opened := ck.openReceiver(m, recv)
		if len(m.Lists) == 0 {
			return opened
		}
		return opened
	case types.SymObject, types.SymClass, types.SymTrait:
		if m.Type != nil {
			return m.Type
		}
		return ck.Ctx.Named(m, nil)
	default:
		if m.Type != nil {
			return m.Type
		}
		return ck.Ctx.Error
	}
}

func (ck *Checker) lookupMemberOrExt(recv *types.Type, name string, recvExpr ast.Expr) *types.Symbol {
	if m := ck.Ctx.LookupMember(recv, name); m != nil {
		return m
	}
	// Companion of a class value (the class used as a constructor) is not a value.
	if recv.Kind == types.TyNamed && recv.Sym != nil && recv.Sym.Companion != nil {
		if m := recv.Sym.Companion.Get(name); m != nil {
			return m
		}
	}
	return ck.findExtension(recv, name)
}

func (ck *Checker) findExtension(recv *types.Type, name string) *types.Symbol {
	recv = types.Repr(recv)
	for _, ext := range ck.exts {
		if ext.Param == nil || ext.Param.Type == nil {
			continue
		}
		for _, m := range ext.Methods {
			if m.Name != name {
				continue
			}
			// Monomorphic extensions compare directly. Generic ones are accepted
			// when the receiver is a subtype of the parameter after substitution
			// is left to applySymbol.
			if len(ext.TParams) == 0 && ck.Ctx.Subtype(recv, ext.Param.Type) {
				return m
			}
			if len(ext.TParams) > 0 {
				return m
			}
		}
	}
	return nil
}

func (ck *Checker) synthApply(e *ast.ApplyExpr) *types.Type {
	if id, ok := e.Fun.(*ast.Ident); ok {
		hits := ck.lookup(id.Name)
		if cls := constructorSym(hits); cls != nil && (preferTerm(hits) == nil || preferTerm(hits).Kind == types.SymClass || preferTerm(hits) == cls) {
			ck.noteSym(e.Fun, cls)
			return ck.applyCtor(e.Span(), cls, e.TypeArgs, e.Args)
		}
		if obj := preferTerm(hits); obj != nil && obj.Kind == types.SymObject {
			apply := obj.Get("apply")
			if apply == nil && obj.Scope != nil {
				apply = firstNamed(obj.Scope.Lookup("apply"), types.SymMethod)
			}
			if apply != nil {
				ck.noteSym(e, apply)
				return ck.applySymbol(e.Span(), apply, obj.Type, e.TypeArgs, e.Args, e.Using)
			}
			if obj.Companion != nil && obj.Companion.IsClassLike() {
				return ck.applyCtor(e.Span(), obj.Companion, e.TypeArgs, e.Args)
			}
		}
	}
	prevSub := ck.subject
	ck.subject = e
	defer func() { ck.subject = prevSub }()
	funT := ck.synth(e.Fun)
	if funT.IsError() {
		return funT
	}
	if sym := ck.syms[e.Fun]; sym != nil && sym.Kind == types.SymMethod {
		recv := ck.receiverType(sym)
		if sel, ok := e.Fun.(*ast.SelectExpr); ok {
			recv = ck.types[sel.X]
		}
		return ck.applySymbol(e.Span(), sym, recv, e.TypeArgs, e.Args, e.Using)
	}
	if funT.Kind == types.TyFun {
		return ck.applyFun(e, funT)
	}
	if funT.Kind == types.TyNamed && funT.Sym != nil && funT.Sym.IsClassLike() {
		return ck.applyCtor(e.Span(), funT.Sym, e.TypeArgs, e.Args)
	}
	ck.errorf(e.Span(), "cannot apply %s", ck.Ctx.Format(funT))
	return ck.Ctx.Error
}

func constructorSym(hits []*types.Symbol) *types.Symbol {
	for _, s := range hits {
		if s.Kind == types.SymClass || s.Kind == types.SymTrait {
			return s
		}
	}
	return nil
}

func firstNamed(hits []*types.Symbol, kind types.SymKind) *types.Symbol {
	for _, s := range hits {
		if s.Kind == kind {
			return s
		}
	}
	return nil
}

func (ck *Checker) applyFun(e *ast.ApplyExpr, fun *types.Type) *types.Type {
	if len(e.Args) != len(fun.Params) {
		ck.errorf(e.Span(), "expected %d argument(s), found %d", len(fun.Params), len(e.Args))
		return ck.Ctx.Error
	}
	for i, a := range e.Args {
		ck.check(a, fun.Params[i])
	}
	return fun.Res
}

func (ck *Checker) applyCtor(sp span.Span, class *types.Symbol, targs []ast.TypeNode, args []ast.Expr) *types.Type {
	params := classParams(class)
	var argTypes []*types.Type
	subst := map[*types.Type]*types.Type{}
	if len(targs) > 0 {
		if len(targs) != len(params) {
			return ck.Ctx.Error
		}
		argTypes = make([]*types.Type, len(targs))
		for i, ta := range targs {
			argTypes[i] = ck.resolveType(ta)
			if params[i].Skolem != nil {
				subst[params[i].Skolem] = argTypes[i]
			}
		}
	} else if len(params) > 0 {
		argTypes = make([]*types.Type, len(params))
		for i, p := range params {
			v := ck.Ctx.Fresh(p.Name, ck.level)
			argTypes[i] = v
			if p.Skolem != nil {
				subst[p.Skolem] = v
			}
		}
	}
	ctor := class.Ctor
	if len(args) != len(ctor) {
		ck.errorf(sp, "%s expects %d argument(s), found %d", class.Name, len(ctor), len(args))
		return ck.Ctx.Error
	}
	for i, a := range args {
		want := substSymType(ck.Ctx, ctor[i].Type, subst)
		got := ck.synth(a)
		if !ck.Ctx.Constrain(got, want) && !ck.Ctx.Subtype(got, types.Repr(want)) {
			ck.mismatch(a.Span(), got, want)
		}
	}
	for i := range argTypes {
		argTypes[i] = ck.Ctx.Instantiate(argTypes[i], true)
	}
	if len(argTypes) == 0 {
		if class.Type != nil {
			return class.Type
		}
		return ck.Ctx.Named(class, nil)
	}
	return ck.Ctx.Named(class, argTypes)
}

func (ck *Checker) applySymbol(sp span.Span, method *types.Symbol, recv *types.Type, targs []ast.TypeNode, args []ast.Expr, using bool) *types.Type {
	_ = using
	subst := ck.inheritSubst(recv, method.Owner)
	fresh := []*types.Type{}
	if len(targs) > 0 {
		if len(targs) != len(method.TParams) {
			return ck.Ctx.Error
		}
		for i, tp := range method.TParams {
			if tp.Skolem != nil {
				subst[tp.Skolem] = ck.resolveType(targs[i])
			}
		}
	} else {
		for _, tp := range method.TParams {
			v := ck.Ctx.Fresh(tp.Name, ck.level)
			fresh = append(fresh, v)
			if tp.Skolem != nil {
				subst[tp.Skolem] = v
			}
		}
	}
	// Fill using clauses from the given scope when the caller did not pass them.
	var valueLists []types.ParamList
	for _, list := range method.Lists {
		if list.Using {
			for _, p := range list.Params {
				want := substSymType(ck.Ctx, p.Type, subst)
				if g, ok := ck.resolveGiven(want, sp); ok {
					if ck.subject != nil {
						ck.evidence[ck.subject] = append(ck.evidence[ck.subject], g)
					}
				} else if !want.IsError() {
					ck.errorf(sp, "no given instance of %s", ck.Ctx.Format(want))
				}
			}
			continue
		}
		valueLists = append(valueLists, list)
	}
	if len(valueLists) == 0 {
		res := method.Result
		res = substSymType(ck.Ctx, res, subst)
		return ck.Ctx.Instantiate(res, true)
	}
	list := valueLists[0]
	if len(args) != len(list.Params) {
		ck.errorf(method.Span, "%s expects %d argument(s), found %d", method.Name, len(list.Params), len(args))
		return ck.Ctx.Error
	}
	for i, a := range args {
		want := substSymType(ck.Ctx, list.Params[i].Type, subst)
		got := ck.synth(a)
		if want != nil && !ck.Ctx.Constrain(got, want) && !ck.Ctx.Subtype(got, types.Repr(want)) {
			ck.mismatch(a.Span(), got, types.Repr(want))
		}
	}
	res := substSymType(ck.Ctx, method.Result, subst)
	return ck.Ctx.Instantiate(res, len(fresh) > 0 || len(method.TParams) > 0)
}

func (ck *Checker) synthTypeApply(e *ast.TypeApplyExpr) *types.Type {
	if id, ok := e.Fun.(*ast.Ident); ok {
		if cls := constructorSym(ck.lookup(id.Name)); cls != nil {
			args := make([]*types.Type, len(e.Args))
			for i, a := range e.Args {
				args[i] = ck.resolveType(a)
			}
			ck.noteSym(e, cls)
			return ck.Ctx.Named(cls, args)
		}
	}
	ck.synth(e.Fun)
	return ck.Ctx.Error
}

func (ck *Checker) checkLambda(e *ast.LambdaExpr, want *types.Type) *types.Type {
	want = types.Repr(want)
	var paramTypes []*types.Type
	var res *types.Type
	if want != nil && want.Kind == types.TyFun {
		if len(want.Params) != len(e.Params) {
			ck.errorf(e.Span(), "expected %d parameter(s)", len(want.Params))
			return ck.Ctx.Error
		}
		paramTypes = want.Params
		res = want.Res
	}
	prev := ck.scope
	ck.scope = types.NewScope(prev, prev.Owner)
	defer func() { ck.scope = prev }()
	pts := make([]*types.Type, len(e.Params))
	for i, p := range e.Params {
		s := ck.Ctx.NewSymbol(p.Name, types.SymTerm)
		s.Flags = types.FlagParam | types.FlagStable
		s.Span = p.Span()
		if p.Type != nil {
			s.Type = ck.resolveType(p.Type)
		} else if i < len(paramTypes) {
			s.Type = paramTypes[i]
		} else {
			ck.errorf(p.Span(), "missing type for %s", p.Name)
			s.Type = ck.Ctx.Error
		}
		pts[i] = s.Type
		ck.scope.Bind(s)
		ck.noteSym(p, s)
	}
	var got *types.Type
	if res != nil {
		got = ck.check(e.Body, res)
	} else {
		got = ck.synth(e.Body)
	}
	return ck.Ctx.Fun(pts, got)
}

func (ck *Checker) synthNew(e *ast.NewExpr) *types.Type {
	if len(e.Parents) == 0 {
		return ck.Ctx.Error
	}
	pt := ck.resolveType(e.Parents[0].Type)
	if pt.Kind != types.TyNamed || pt.Sym == nil {
		ck.errorf(e.Span(), "new expects a class")
		return ck.Ctx.Error
	}
	ctor := pt.Sym.Ctor
	args := e.Parents[0].Args
	if len(args) != len(ctor) {
		ck.errorf(e.Span(), "%s expects %d argument(s)", pt.Sym.Name, len(ctor))
		return ck.Ctx.Error
	}
	sub := classArgMap(pt)
	for i, a := range args {
		ck.check(a, substSymType(ck.Ctx, ctor[i].Type, sub))
	}
	if e.Template != nil {
		for _, st := range e.Template.Stats {
			ck.checkStmt(st)
		}
	}
	return pt
}

func (ck *Checker) synthSuper(e *ast.SuperExpr) *types.Type {
	if ck.this == nil || ck.this.Owner == nil {
		ck.errorf(e.Span(), "super is not in scope")
		return ck.Ctx.Error
	}
	parents := ck.this.Owner.Parents
	if len(parents) == 0 {
		return ck.Ctx.Top
	}
	if e.Mix != "" {
		for _, p := range parents {
			if p.Kind == types.TyNamed && p.Sym != nil && p.Sym.Name == e.Mix {
				return p
			}
		}
	}
	return parents[0]
}

func (ck *Checker) synthInterp(e *ast.InterpExpr) *types.Type {
	for _, h := range e.Holes {
		ck.synth(h)
	}
	return ck.Ctx.StringType()
}

func (ck *Checker) synthFor(e *ast.ForExpr) *types.Type {
	rewritten := ck.desugarFor(e)
	if rewritten == nil {
		return ck.Ctx.Error
	}
	ck.rewrites[e] = rewritten
	return ck.synth(rewritten)
}

func (ck *Checker) desugarFor(e *ast.ForExpr) ast.Expr {
	if len(e.Enums) == 0 {
		ck.errorf(e.Span(), "empty for comprehension")
		return nil
	}
	return ck.desugarEnums(e.Enums, e.Body, e.Yield, e.Span())
}

func (ck *Checker) desugarEnums(enums []*ast.Enumerator, body ast.Expr, yield bool, sp span.Span) ast.Expr {
	if len(enums) == 0 {
		return body
	}
	en := enums[0]
	rest := enums[1:]
	if en.Guard != nil && en.Pat == nil {
		// if g; rest  =>  withFilter on the previous generator. A leading guard
		// is a type error.
		ck.errorf(en.Span(), "guard needs a generator")
		return nil
	}
	inner := ck.desugarEnums(rest, body, yield, sp)
	method := "map"
	if len(rest) > 0 || !yield {
		method = "flatMap"
	}
	if !yield && len(rest) == 0 {
		method = "foreach"
	}
	lam := &ast.LambdaExpr{
		S:      sp,
		Params: []*ast.Param{{S: en.Pat.Span(), Name: binderName(en.Pat)}},
		Body:   inner,
	}
	// Patterns other than a bare name are bound by a match inside the lambda.
	if _, ok := en.Pat.(*ast.BindPat); !ok {
		if _, ok := en.Pat.(*ast.WildPat); !ok {
			lam.Params[0].Name = "$g"
			lam.Body = &ast.MatchExpr{
				S: sp, Scrut: &ast.Ident{S: en.Pat.Span(), Name: "$g"},
				Cases: []*ast.CaseClause{{S: sp, Pat: en.Pat, Body: inner}},
			}
		}
	}
	call := &ast.ApplyExpr{
		S:    sp,
		Fun:  &ast.SelectExpr{S: sp, X: en.Expr, Name: method},
		Args: []ast.Expr{lam},
	}
	if en.Guard != nil {
		filt := &ast.ApplyExpr{
			S:   sp,
			Fun: &ast.SelectExpr{S: sp, X: en.Expr, Name: "withFilter"},
			Args: []ast.Expr{&ast.LambdaExpr{
				S:      sp,
				Params: []*ast.Param{{Name: binderName(en.Pat), S: en.Pat.Span()}},
				Body:   en.Guard,
			}},
		}
		call.Fun = &ast.SelectExpr{S: sp, X: filt, Name: method}
	}
	return call
}

func binderName(p ast.Pattern) string {
	if b, ok := p.(*ast.BindPat); ok && b.Pat == nil {
		return b.Name
	}
	return "$x"
}
