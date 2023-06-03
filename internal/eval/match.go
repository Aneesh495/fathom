package eval

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/types"
)

func (m *Machine) match(e *ast.MatchExpr) Value {
	scrut := m.expr(e.Scrut)
	if m.err != nil || m.ret {
		return m.retVal
	}
	for _, c := range e.Cases {
		binds, ok := m.tryPat(c.Pat, scrut)
		if !ok {
			continue
		}
		if c.Guard != nil {
			saved := m.env
			m.env = newEnv(saved)
			for s, v := range binds {
				m.env.set(s, v)
			}
			g := m.expr(c.Guard)
			m.env = saved
			if m.err != nil || m.ret {
				return m.retVal
			}
			if !g.Truth() {
				continue
			}
		}
		saved := m.env
		m.env = newEnv(saved)
		for s, v := range binds {
			m.env.set(s, v)
		}
		v := m.expr(c.Body)
		m.env = saved
		return v
	}
	return m.fail("match error")
}

func (m *Machine) tryPat(p ast.Pattern, v Value) (map[*types.Symbol]Value, bool) {
	if p == nil {
		return nil, false
	}
	switch p := p.(type) {
	case *ast.WildPat:
		return map[*types.Symbol]Value{}, true
	case *ast.BindPat:
		sym := m.info.Syms[p]
		binds := map[*types.Symbol]Value{}
		if sym != nil {
			binds[sym] = v
		}
		if p.Pat != nil {
			inner, ok := m.tryPat(p.Pat, v)
			if !ok {
				return nil, false
			}
			for s, val := range inner {
				binds[s] = val
			}
		}
		return binds, true
	case *ast.LitPat:
		lit := m.literal(p.Lit)
		if m.err != nil {
			m.err = nil
			return nil, false
		}
		if equal(lit, v) {
			return map[*types.Symbol]Value{}, true
		}
		return nil, false
	case *ast.TypedPat:
		return m.tryPat(p.Pat, v)
	case *ast.TuplePat:
		if v.Tag != VObj || v.Obj == nil {
			return nil, false
		}
		if len(v.Obj.Fields) < len(p.Pats) {
			return nil, false
		}
		all := map[*types.Symbol]Value{}
		for i, sub := range p.Pats {
			binds, ok := m.tryPat(sub, v.Obj.Fields[i])
			if !ok {
				return nil, false
			}
			for s, val := range binds {
				all[s] = val
			}
		}
		return all, true
	case *ast.AltPat:
		for _, a := range p.Pats {
			if binds, ok := m.tryPat(a, v); ok {
				return binds, true
			}
		}
		return nil, false
	case *ast.CtorPat:
		sym := m.info.Syms[p]
		if sym == nil || v.Tag != VObj || v.Obj == nil || v.Obj.Class != sym {
			return nil, false
		}
		if sym.Kind == types.SymObject {
			return map[*types.Symbol]Value{}, true
		}
		all := map[*types.Symbol]Value{}
		for i, sub := range p.Pats {
			if i >= len(v.Obj.Fields) {
				return nil, false
			}
			binds, ok := m.tryPat(sub, v.Obj.Fields[i])
			if !ok {
				return nil, false
			}
			for s, val := range binds {
				all[s] = val
			}
		}
		return all, true
	default:
		return nil, false
	}
}
