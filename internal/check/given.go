package check

import (
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/types"
)

// resolveGiven searches lexical givens and the companions of the target type.
// A candidate is more specific when its instantiated type is a subtype of the
// other candidate's type. Two successful incomparable candidates are an error.
// The open set stops a given from requiring itself.
func (ck *Checker) resolveGiven(want *types.Type, sp span.Span) (*types.Symbol, bool) {
	want = types.Repr(want)
	if want == nil || want.IsError() {
		return nil, false
	}
	if ck.givenOpen == nil {
		ck.givenOpen = map[*types.Type]bool{}
	}
	if ck.givenOpen[want] {
		return nil, false
	}
	ck.givenOpen[want] = true
	defer delete(ck.givenOpen, want)

	cands := ck.gatherGivens(want)
	type success struct {
		sym *types.Symbol
		t   *types.Type
	}
	var ok []success
	for _, cand := range cands {
		got, good := ck.tryGiven(cand, want)
		if good {
			ok = append(ok, success{sym: cand, t: got})
		}
	}
	if len(ok) == 0 {
		return nil, false
	}
	best := ok[0]
	ambiguous := false
	for _, other := range ok[1:] {
		aSub := ck.Ctx.Subtype(best.t, other.t)
		bSub := ck.Ctx.Subtype(other.t, best.t)
		switch {
		case bSub && !aSub:
			best = other
			ambiguous = false
		case aSub && !bSub:
			// best stays
		case aSub && bSub:
			// equivalent, keep the earlier one
		default:
			ambiguous = true
		}
	}
	if ambiguous {
		ck.errorf(sp, "ambiguous given instances for %s", ck.Ctx.Format(want))
		return nil, false
	}
	return best.sym, true
}

func (ck *Checker) gatherGivens(want *types.Type) []*types.Symbol {
	seen := map[*types.Symbol]bool{}
	var out []*types.Symbol
	add := func(s *types.Symbol) {
		if s == nil || seen[s] || !s.Has(types.FlagGiven) && s.Kind != types.SymTerm {
			return
		}
		if !s.Has(types.FlagGiven) {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for sc := ck.scope; sc != nil; sc = sc.Parent {
		for _, name := range sc.Names() {
			for _, s := range sc.Local(name) {
				add(s)
			}
		}
	}
	for _, s := range ck.given {
		add(s)
	}
	for _, comp := range companionsOf(want) {
		if comp == nil || comp.Scope == nil {
			continue
		}
		for _, name := range comp.Scope.Names() {
			for _, s := range comp.Scope.Local(name) {
				add(s)
			}
		}
		for _, m := range comp.Members {
			add(m)
		}
	}
	return out
}

func companionsOf(t *types.Type) []*types.Symbol {
	t = types.Repr(t)
	if t == nil {
		return nil
	}
	var out []*types.Symbol
	switch t.Kind {
	case types.TyNamed:
		if t.Sym != nil && t.Sym.Companion != nil {
			out = append(out, t.Sym.Companion)
		}
		for _, a := range t.Args {
			out = append(out, companionsOf(a)...)
		}
	case types.TyFun:
		for _, p := range t.Params {
			out = append(out, companionsOf(p)...)
		}
		out = append(out, companionsOf(t.Res)...)
	case types.TyUnion, types.TyInter:
		out = append(out, companionsOf(t.L)...)
		out = append(out, companionsOf(t.R)...)
	}
	return out
}

func (ck *Checker) tryGiven(sym *types.Symbol, want *types.Type) (*types.Type, bool) {
	if sym.Result == nil && sym.Type == nil {
		return nil, false
	}
	subst := map[*types.Type]*types.Type{}
	for _, tp := range sym.TParams {
		if tp.Skolem == nil {
			continue
		}
		subst[tp.Skolem] = ck.Ctx.Fresh(tp.Name, ck.level)
	}
	got := sym.Result
	if got == nil {
		got = sym.Type
	}
	got = substSymType(ck.Ctx, got, subst)
	if !ck.Ctx.Constrain(got, want) && !ck.Ctx.Subtype(types.Repr(got), want) {
		return nil, false
	}
	for _, list := range sym.Lists {
		if !list.Using {
			continue
		}
		for _, p := range list.Params {
			need := substSymType(ck.Ctx, p.Type, subst)
			need = ck.Ctx.Instantiate(need, false)
			if _, ok := ck.resolveGiven(need, sym.Span); !ok {
				return nil, false
			}
		}
	}
	got = ck.Ctx.Instantiate(got, true)
	if got.IsError() {
		return nil, false
	}
	return got, ck.Ctx.Subtype(got, want) || ck.Ctx.Constrain(got, want)
}
