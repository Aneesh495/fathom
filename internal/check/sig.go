package check

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/types"
)

func (ck *Checker) sigFile(f *ast.File) {
	scope := ck.Root
	if f.Package != nil && len(f.Package.Parts) > 0 {
		scope = ck.enterPackage(f.Package.Parts, ck.Root)
	}
	prev := ck.scope
	ck.scope = scope
	for _, st := range f.Stats {
		ck.sigStmt(st)
	}
	ck.scope = prev
}

func (ck *Checker) sigStmt(st ast.Stmt) {
	switch n := st.(type) {
	case *ast.ClassDecl:
		ck.sigClass(n)
	case *ast.DefDecl:
		ck.sigDef(n)
	case *ast.ValDecl:
		ck.sigVal(n)
	case *ast.TypeDecl:
		ck.sigTypeDecl(n)
	case *ast.GivenDecl:
		ck.sigGiven(n)
	case *ast.ExtDecl:
		ck.sigExt(n)
	}
}

func (ck *Checker) sigExt(n *ast.ExtDecl) {
	if n.Param != nil {
		if s := ck.syms[n.Param]; s != nil && n.Param.Type != nil {
			s.Type = ck.resolveType(n.Param.Type)
		}
	}
	if n.Body == nil {
		return
	}
	for _, st := range n.Body.Stats {
		ck.sigStmt(st)
	}
}

func (ck *Checker) sigClass(n *ast.ClassDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	prevScope, prevThis := ck.scope, ck.this
	ck.scope = sym.Scope
	defer func() { ck.scope, ck.this = prevScope, prevThis }()

	ck.finishTypeParams(n.TParams)
	params := classParams(sym)
	args := make([]*types.Type, len(params))
	for i, p := range params {
		args[i] = p.Skolem
	}
	if sym == ck.Ctx.Nothing {
		sym.Type = ck.Ctx.Bot
	} else if len(args) == 0 && sym == ck.Ctx.Any {
		sym.Type = ck.Ctx.Top
	} else {
		sym.Type = ck.Ctx.Named(sym, args)
	}
	th := sym.Get("this")
	if th == nil {
		th = ck.Ctx.NewSymbol("this", types.SymTerm)
		th.Flags = types.FlagSynthetic | types.FlagStable | types.FlagParam
		sym.AddMember(th)
		sym.Scope.Bind(th)
	}
	th.Type = sym.Type
	ck.this = th

	if len(n.Parents) == 0 {
		if sym != ck.Ctx.Any && sym != ck.Ctx.Nothing {
			sym.Parents = []*types.Type{ck.Ctx.Named(ck.Ctx.AnyRef, nil)}
		}
	} else {
		for _, p := range n.Parents {
			pt := ck.resolveType(p.Type)
			sym.Parents = append(sym.Parents, pt)
			if p.HasArgs {
				ck.checkParentArgs(p, pt)
			}
		}
	}
	sym.Lin = nil

	for i, p := range n.Params {
		if i < len(sym.Ctor) && p.Type != nil {
			sym.Ctor[i].Type = ck.resolveType(p.Type)
		}
	}
	ck.sigEnumCases(n, sym)
	if n.Body != nil {
		saved := ck.scope
		for _, st := range n.Body.Stats {
			ck.scope = sym.Scope
			ck.sigStmt(st)
		}
		ck.scope = saved
	}
	if sym.Companion != nil && sym.Companion.Type == nil {
		sym.Companion.Parents = []*types.Type{ck.Ctx.Named(ck.Ctx.AnyRef, nil)}
		sym.Companion.Type = ck.Ctx.Named(sym.Companion, nil)
		sym.Companion.Lin = nil
	}
}

func (ck *Checker) checkParentArgs(p *ast.Parent, pt *types.Type) {
	// Constructor arguments of a parent are checked in the body pass, once
	// the parent signature is known. Remember the type for that pass.
	ck.noteType(p, pt)
}

func (ck *Checker) sigEnumCases(n *ast.ClassDecl, parent *types.Symbol) {
	pparams := classParams(parent)
	for _, c := range n.Cases {
		sym := ck.syms[c]
		if sym == nil {
			continue
		}
		if sym.Kind == types.SymObject {
			args := make([]*types.Type, len(pparams))
			for i := range args {
				args[i] = ck.Ctx.Bot
			}
			var pt *types.Type
			if len(args) == 0 {
				pt = parent.Type
			} else {
				pt = ck.Ctx.Named(parent, args)
			}
			sym.Parents = []*types.Type{pt}
			sym.Type = pt
			sym.Lin = nil
			continue
		}
		cps := classParams(sym)
		args := make([]*types.Type, len(cps))
		for i, p := range cps {
			args[i] = p.Skolem
		}
		sym.Type = ck.Ctx.Named(sym, args)
		sym.Parents = []*types.Type{ck.Ctx.Named(parent, args)}
		sym.Lin = nil
		prev := ck.scope
		ck.scope = sym.Scope
		for i, p := range c.Params {
			if i < len(sym.Ctor) && p.Type != nil {
				sym.Ctor[i].Type = ck.resolveType(p.Type)
			}
		}
		ck.scope = prev
		th := ck.Ctx.NewSymbol("this", types.SymTerm)
		th.Flags = types.FlagSynthetic | types.FlagStable | types.FlagParam
		th.Type = sym.Type
		sym.AddMember(th)
		if sym.Scope != nil {
			sym.Scope.Bind(th)
		}
	}
}

func (ck *Checker) sigDef(n *ast.DefDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	prev := ck.scope
	ck.scope = sym.Scope
	if ck.this == nil && sym.Owner != nil {
		ck.this = sym.Owner.Get("this")
	}
	ck.finishTypeParams(n.TParams)
	for _, list := range sym.Lists {
		for _, p := range list.Params {
			pa, _ := p.AST.(*ast.Param)
			if pa != nil && pa.Type != nil {
				p.Type = ck.resolveType(pa.Type)
			}
		}
	}
	if n.Result != nil {
		sym.Result = ck.resolveType(n.Result)
	}
	ck.finishMethodType(sym)
	ck.scope = prev
}

func (ck *Checker) finishMethodType(sym *types.Symbol) {
	if sym == nil || sym.Result == nil {
		return
	}
	// The value view of a method is the function type of its first non-using list.
	for _, list := range sym.Lists {
		if list.Using {
			continue
		}
		params := make([]*types.Type, len(list.Params))
		for i, p := range list.Params {
			params[i] = p.Type
			if params[i] == nil {
				params[i] = ck.Ctx.Error
			}
		}
		sym.Type = ck.Ctx.Fun(params, sym.Result)
		return
	}
	sym.Type = sym.Result
}

func (ck *Checker) sigVal(n *ast.ValDecl) {
	sym := ck.syms[n]
	if sym == nil || n.Type == nil {
		return
	}
	sym.Type = ck.resolveType(n.Type)
}

func (ck *Checker) sigTypeDecl(n *ast.TypeDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	if ck.busy[sym] {
		ck.errorf(n.Span(), "cyclic type %s", n.Name)
		sym.Type = ck.Ctx.Error
		return
	}
	ck.busy[sym] = true
	prev := ck.scope
	ck.scope = sym.Scope
	ck.finishTypeParams(n.TParams)
	if n.Body != nil {
		sym.Type = ck.resolveType(n.Body)
		sym.Result = sym.Type
	} else {
		lo, hi := ck.Ctx.Bot, ck.Ctx.Top
		if n.Lo != nil {
			lo = ck.resolveType(n.Lo)
		}
		if n.Hi != nil {
			hi = ck.resolveType(n.Hi)
		}
		sym.Type = lo
		sym.Result = hi
		sym.Skolem = ck.Ctx.Skolem(n.Name, lo, hi)
	}
	if sym.Has(types.FlagOpaque) && sym.Skolem == nil {
		sym.Skolem = ck.Ctx.Skolem(n.Name, ck.Ctx.Bot, ck.Ctx.Top)
	}
	ck.busy[sym] = false
	ck.scope = prev
}

func (ck *Checker) sigGiven(n *ast.GivenDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	prev := ck.scope
	ck.scope = sym.Scope
	ck.finishTypeParams(n.TParams)
	for _, list := range sym.Lists {
		for _, p := range list.Params {
			pa, _ := p.AST.(*ast.Param)
			if pa != nil && pa.Type != nil {
				p.Type = ck.resolveType(pa.Type)
			}
		}
	}
	if n.Type != nil {
		sym.Result = ck.resolveType(n.Type)
		sym.Type = sym.Result
	}
	if n.BodyTemplate != nil {
		for _, st := range n.BodyTemplate.Stats {
			ck.scope = sym.Scope
			ck.sigStmt(st)
		}
	}
	ck.scope = prev
}

func (ck *Checker) finishTypeParams(list []*ast.TypeParam) {
	for _, tp := range list {
		if tp == nil {
			continue
		}
		s := ck.syms[tp]
		if s == nil || s.Skolem == nil {
			continue
		}
		if tp.Lo != nil {
			s.Skolem.Lo = ck.resolveType(tp.Lo)
		}
		if tp.Hi != nil {
			s.Skolem.Hi = ck.resolveType(tp.Hi)
		}
	}
}

func (ck *Checker) bodyFile(f *ast.File) {
	scope := ck.Root
	if f.Package != nil && len(f.Package.Parts) > 0 {
		scope = ck.enterPackage(f.Package.Parts, ck.Root)
	}
	prev := ck.scope
	ck.scope = scope
	for _, st := range f.Stats {
		ck.checkStmt(st)
	}
	ck.scope = prev
}

func (ck *Checker) checkStmt(st ast.Stmt) {
	switch n := st.(type) {
	case *ast.ClassDecl:
		ck.checkClass(n)
	case *ast.DefDecl:
		ck.checkDef(n)
	case *ast.ValDecl:
		ck.checkVal(n)
	case *ast.GivenDecl:
		ck.checkGiven(n)
	case *ast.ExprStmt:
		ck.synth(n.X)
	case *ast.ExtDecl:
		if n.Body != nil {
			for _, st := range n.Body.Stats {
				ck.checkStmt(st)
			}
		}
	case *ast.TypeDecl, *ast.Import:
	}
}

func (ck *Checker) checkClass(n *ast.ClassDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	prevScope, prevThis := ck.scope, ck.this
	ck.scope = sym.Scope
	ck.this = sym.Get("this")
	defer func() { ck.scope, ck.this = prevScope, prevThis }()
	for _, p := range n.Parents {
		if !p.HasArgs {
			continue
		}
		pt := ck.types[p]
		if pt == nil {
			pt = ck.resolveType(p.Type)
		}
		ck.applyParent(p, pt)
	}
	if n.Body != nil {
		for _, st := range n.Body.Stats {
			ck.scope = sym.Scope
			ck.this = sym.Get("this")
			ck.checkStmt(st)
		}
	}
}

func (ck *Checker) applyParent(p *ast.Parent, pt *types.Type) {
	pt = types.Repr(pt)
	if pt == nil || pt.Kind != types.TyNamed || pt.Sym == nil {
		return
	}
	ctor := pt.Sym.Ctor
	if len(p.Args) != len(ctor) {
		ck.errorf(p.Span(), "%s constructor expects %d argument(s)", pt.Sym.Name, len(ctor))
		return
	}
	sub := classArgMap(pt)
	for i, arg := range p.Args {
		want := ctor[i].Type
		want = substSymType(ck.Ctx, want, sub)
		ck.check(arg, want)
	}
}

func (ck *Checker) checkDef(n *ast.DefDecl) {
	sym := ck.syms[n]
	if sym == nil || n.Body == nil {
		return
	}
	prevScope, prevThis, prevRet := ck.scope, ck.this, ck.ret
	ck.scope = sym.Scope
	ck.ret = sym.Result
	if sym.Owner != nil && sym.Owner.IsClassLike() {
		ck.this = sym.Owner.Get("this")
	}
	defer func() { ck.scope, ck.this, ck.ret = prevScope, prevThis, prevRet }()
	if sym.Result != nil {
		ck.check(n.Body, sym.Result)
		return
	}
	got := ck.synth(n.Body)
	sym.Result = got
	ck.finishMethodType(sym)
	ck.noteType(n, got)
}

func (ck *Checker) checkVal(n *ast.ValDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	if n.Pat != nil {
		got := ck.synth(n.Body)
		ck.bindPattern(n.Pat, got, true)
		ck.noteType(n, got)
		return
	}
	if n.Body == nil {
		if sym.Type == nil {
			ck.errorf(n.Span(), "%s needs a type or an initializer", n.Name)
			sym.Type = ck.Ctx.Error
		}
		return
	}
	if sym.Type != nil {
		ck.check(n.Body, sym.Type)
		ck.noteType(n, sym.Type)
		return
	}
	sym.Type = ck.synth(n.Body)
	ck.noteType(n, sym.Type)
}

func (ck *Checker) checkGiven(n *ast.GivenDecl) {
	sym := ck.syms[n]
	if sym == nil {
		return
	}
	prev := ck.scope
	ck.scope = sym.Scope
	defer func() { ck.scope = prev }()
	if n.Body != nil && sym.Type != nil {
		ck.check(n.Body, sym.Type)
		return
	}
	if n.BodyTemplate != nil {
		for _, st := range n.BodyTemplate.Stats {
			ck.scope = sym.Scope
			ck.checkStmt(st)
		}
	}
}
