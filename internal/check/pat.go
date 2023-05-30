package check

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/types"
)

func (ck *Checker) synthMatch(e *ast.MatchExpr, want *types.Type) *types.Type {
	scrut := ck.synth(e.Scrut)
	if len(e.Cases) == 0 {
		ck.errorf(e.Span(), "match has no cases")
		return ck.Ctx.Error
	}
	var acc *types.Type
	for _, c := range e.Cases {
		t := ck.synthCase(scrut, c, want)
		if acc == nil {
			acc = t
		} else if want == nil {
			acc = ck.Ctx.Lub(acc, t)
		}
	}
	ck.checkExhaustive(e, scrut)
	if want != nil {
		return want
	}
	if acc == nil {
		return ck.Ctx.Error
	}
	return acc
}

func (ck *Checker) synthCase(scrut *types.Type, c *ast.CaseClause, want *types.Type) *types.Type {
	if c == nil {
		return ck.Ctx.Error
	}
	prev := ck.scope
	ck.scope = types.NewScope(prev, prev.Owner)
	defer func() { ck.scope = prev }()
	ck.bindPattern(c.Pat, scrut, true)
	if c.Guard != nil {
		ck.check(c.Guard, ck.Ctx.BooleanType())
	}
	if want != nil {
		return ck.check(c.Body, want)
	}
	return ck.synth(c.Body)
}

// bindPattern introduces bindings for a pattern checked against t.
// bind false is used by exhaustiveness, which only cares about coverage.
func (ck *Checker) bindPattern(p ast.Pattern, t *types.Type, bind bool) bool {
	if p == nil {
		return false
	}
	t = types.Repr(t)
	switch p := p.(type) {
	case *ast.WildPat:
		return true
	case *ast.BindPat:
		if bind && p.Pat == nil {
			s := ck.Ctx.NewSymbol(p.Name, types.SymTerm)
			s.Flags = types.FlagStable | types.FlagParam
			s.Type = t
			s.Span = p.Span()
			ck.scope.Bind(s)
			ck.noteSym(p, s)
			return true
		}
		if bind && p.Name != "" && p.Name != "_" {
			s := ck.Ctx.NewSymbol(p.Name, types.SymTerm)
			s.Flags = types.FlagStable | types.FlagParam
			s.Type = t
			s.Span = p.Span()
			ck.scope.Bind(s)
			ck.noteSym(p, s)
		}
		return ck.bindPattern(p.Pat, t, bind)
	case *ast.LitPat:
		lt := ck.litType(p.Lit)
		if !ck.Ctx.Subtype(lt, t) && !ck.Ctx.Subtype(t, ck.widenLit(lt)) && t.Kind != types.TyError {
			// A literal pattern is allowed when the scrutinee's widened type
			// matches the literal's base type.
			if !ck.Ctx.Subtype(ck.widenLit(lt), t) && !sameBase(ck, lt, t) {
				ck.errorf(p.Span(), "pattern %s is not a subtype of %s", ck.Ctx.Format(lt), ck.Ctx.Format(t))
				return false
			}
		}
		return true
	case *ast.TypedPat:
		pt := ck.resolveType(p.Type)
		if t.Kind != types.TyError && pt.Kind != types.TyError && ck.Ctx.Glb(t, pt).Kind == types.TyBot && !ck.Ctx.Subtype(pt, t) && !ck.Ctx.Subtype(t, pt) {
			ck.errorf(p.Span(), "pattern type %s is disjoint from %s", ck.Ctx.Format(pt), ck.Ctx.Format(t))
		}
		return ck.bindPattern(p.Pat, pt, bind)
	case *ast.TuplePat:
		return ck.bindTuplePat(p, t, bind)
	case *ast.CtorPat:
		return ck.bindCtorPat(p, t, bind)
	case *ast.AltPat:
		ok := true
		for _, a := range p.Pats {
			if !ck.bindPattern(a, t, bind) {
				ok = false
			}
		}
		return ok
	default:
		ck.errorf(p.Span(), "unsupported pattern")
		return false
	}
}

func (ck *Checker) widenLit(t *types.Type) *types.Type {
	t = types.Repr(t)
	if t == nil || t.Kind != types.TyLiteral {
		return t
	}
	switch t.Lit {
	case types.LitInt:
		return ck.Ctx.IntType()
	case types.LitFloat:
		return ck.Ctx.DoubleType()
	case types.LitBool:
		return ck.Ctx.BooleanType()
	case types.LitString:
		return ck.Ctx.StringType()
	case types.LitChar:
		return ck.Ctx.CharType()
	case types.LitUnit:
		return ck.Ctx.UnitType()
	default:
		return ck.Ctx.NullType()
	}
}

func sameBase(ck *Checker, lit, scrut *types.Type) bool {
	return ck.Ctx.Subtype(ck.widenLit(lit), scrut) || ck.Ctx.Subtype(scrut, ck.widenLit(lit))
}

func (ck *Checker) bindTuplePat(p *ast.TuplePat, t *types.Type, bind bool) bool {
	var class *types.Symbol
	switch len(p.Pats) {
	case 2:
		class = ck.Ctx.Tuple2
	case 3:
		class = ck.Ctx.Tuple3
	case 4:
		class = ck.Ctx.Tuple4
	default:
		ck.errorf(p.Span(), "tuple pattern has %d elements", len(p.Pats))
		return false
	}
	if t.Kind == types.TyNamed && t.Sym == class && len(t.Args) == len(p.Pats) {
		ok := true
		for i, sub := range p.Pats {
			if !ck.bindPattern(sub, t.Args[i], bind) {
				ok = false
			}
		}
		return ok
	}
	ck.errorf(p.Span(), "expected %s, found %s", class.Name, ck.Ctx.Format(t))
	return false
}

func (ck *Checker) bindCtorPat(p *ast.CtorPat, scrut *types.Type, bind bool) bool {
	sym := ck.ctorSymbol(p.Ctor)
	if sym == nil {
		ck.errorf(p.Span(), "not a constructor")
		return false
	}
	ck.noteSym(p, sym)
	if scrut.Kind == types.TyNamed && scrut.Sym != nil && sym.Kind == types.SymClass {
		// scrut should be a supertype of the case, or the case itself.
		if !ck.Ctx.Subtype(sym.Type, scrut) && !ck.Ctx.Subtype(scrut, sym.Type) && scrut.Sym != sym && !sharesParent(sym, scrut.Sym) {
			ck.errorf(p.Span(), "%s is not a case of %s", sym.Name, ck.Ctx.Format(scrut))
		}
	}
	fields := sym.Ctor
	if sym.Kind == types.SymObject {
		if p.HasArgs && len(p.Pats) > 0 {
			ck.errorf(p.Span(), "%s does not take arguments", sym.Name)
			return false
		}
		return true
	}
	if len(p.Pats) != len(fields) {
		ck.errorf(p.Span(), "%s expects %d pattern(s)", sym.Name, len(fields))
		return false
	}
	sub := ck.caseSubst(sym, scrut)
	ok := true
	for i, subp := range p.Pats {
		ft := fields[i].Type
		ft = substSymType(ck.Ctx, ft, sub)
		if ft == nil {
			ft = ck.Ctx.Error
		}
		if !ck.bindPattern(subp, ft, bind) {
			ok = false
		}
	}
	return ok
}

func sharesParent(caseSym, scrut *types.Symbol) bool {
	if caseSym == nil || scrut == nil {
		return false
	}
	for _, p := range caseSym.Parents {
		if p != nil && p.Kind == types.TyNamed && p.Sym == scrut {
			return true
		}
	}
	return caseSym == scrut
}

func (ck *Checker) ctorSymbol(e ast.Expr) *types.Symbol {
	switch e := e.(type) {
	case *ast.Ident:
		hits := ck.lookup(e.Name)
		if s := constructorSym(hits); s != nil {
			return s
		}
		if s := preferTerm(hits); s != nil && (s.Kind == types.SymObject || s.Kind == types.SymClass) {
			return s
		}
		return nil
	case *ast.SelectExpr:
		recv := ck.synth(e.X)
		if recv.Kind == types.TyNamed && recv.Sym != nil {
			if m := recv.Sym.Get(e.Name); m != nil {
				return m
			}
			if recv.Sym.Companion != nil {
				if m := recv.Sym.Companion.Get(e.Name); m != nil {
					return m
				}
			}
		}
		if sym := ck.syms[e.X]; sym != nil {
			if m := sym.Get(e.Name); m != nil {
				return m
			}
			if sym.Companion != nil {
				return sym.Companion.Get(e.Name)
			}
		}
		return nil
	default:
		return nil
	}
}

func (ck *Checker) checkExhaustive(e *ast.MatchExpr, scrut *types.Type) {
	scrut = types.Repr(scrut)
	if scrut == nil || scrut.IsError() || scrut.Kind == types.TyBot {
		return
	}
	if coversAll(e.Cases) {
		return
	}
	if scrut == ck.Ctx.BooleanType() || (scrut.Kind == types.TyNamed && scrut.Sym == ck.Ctx.Boolean) {
		hasT, hasF := false, false
		for _, c := range e.Cases {
			if litTrue(c.Pat) {
				hasT = true
			}
			if litFalse(c.Pat) {
				hasF = true
			}
		}
		if !hasT || !hasF {
			missing := "false"
			if hasF {
				missing = "true"
			}
			ck.errorf(e.Span(), "match may not be exhaustive. missing: %s", missing)
		}
		return
	}
	if scrut.Kind != types.TyNamed || scrut.Sym == nil || !scrut.Sym.Has(types.FlagSealed) {
		return
	}
	covered := map[*types.Symbol]bool{}
	for _, c := range e.Cases {
		ck.noteCovered(c.Pat, covered)
	}
	var missing []string
	seenMiss := map[string]bool{}
	for _, sub := range directSubs(scrut.Sym) {
		if covered[sub] || seenMiss[sub.Name] {
			continue
		}
		seenMiss[sub.Name] = true
		missing = append(missing, sub.Name)
	}
	if len(missing) > 0 {
		msg := missing[0]
		for _, m := range missing[1:] {
			msg += ", " + m
		}
		ck.errorf(e.Span(), "match may not be exhaustive. missing: %s", msg)
	}
}

func coversAll(cases []*ast.CaseClause) bool {
	for _, c := range cases {
		if c.Guard != nil {
			continue
		}
		if isIrrefutable(c.Pat) {
			return true
		}
	}
	return false
}

func isIrrefutable(p ast.Pattern) bool {
	switch p := p.(type) {
	case *ast.WildPat:
		return true
	case *ast.BindPat:
		return p.Pat == nil || isIrrefutable(p.Pat)
	case *ast.TypedPat:
		return false
	case *ast.TuplePat:
		for _, s := range p.Pats {
			if !isIrrefutable(s) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (ck *Checker) noteCovered(p ast.Pattern, covered map[*types.Symbol]bool) {
	switch p := p.(type) {
	case *ast.CtorPat:
		if s := ck.syms[p]; s != nil {
			covered[s] = true
		}
	case *ast.AltPat:
		for _, a := range p.Pats {
			ck.noteCovered(a, covered)
		}
	case *ast.BindPat:
		if p.Pat != nil {
			ck.noteCovered(p.Pat, covered)
		}
	}
}

func litTrue(p ast.Pattern) bool {
	l, ok := p.(*ast.LitPat)
	return ok && l.Lit != nil && l.Lit.Kind == ast.LitBool && l.Lit.Text == "true"
}

func litFalse(p ast.Pattern) bool {
	l, ok := p.(*ast.LitPat)
	return ok && l.Lit != nil && l.Lit.Kind == ast.LitBool && l.Lit.Text == "false"
}

func directSubs(sym *types.Symbol) []*types.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*types.Symbol
	for _, name := range sym.Scope.Names() {
		for _, s := range sym.Scope.Local(name) {
			if s.Has(types.FlagCase) {
				out = append(out, s)
			}
		}
	}
	if sym.Companion != nil && sym.Companion.Scope != nil {
		for _, name := range sym.Companion.Scope.Names() {
			for _, s := range sym.Companion.Scope.Local(name) {
				if s.Has(types.FlagCase) {
					out = append(out, s)
				}
			}
		}
	}
	return out
}
