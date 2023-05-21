package types

// pair is one subtyping goal. Pointer identity is meaningful because structural
// types are interned and variables are unique.
type pair struct {
	sub, super *Type
}

type subState struct {
	fuel   int
	assume map[pair]struct{}
	// opened records type selections currently being exposed, so a bound that
	// mentions itself cannot diverge.
	opened map[*Type]struct{}
}

// Subtype reports whether sub is a subtype of super.
// The judgment is coinductive on the assumption stack and fails closed when
// fuel is exhausted.
func (c *Context) Subtype(sub, super *Type) bool {
	if c == nil {
		return false
	}
	fuel := c.Fuel
	if fuel <= 0 {
		fuel = 96
	}
	st := &subState{fuel: fuel, assume: map[pair]struct{}{}, opened: map[*Type]struct{}{}}
	return c.sub(Repr(sub), Repr(super), st)
}

func (c *Context) sub(sub, super *Type, st *subState) bool {
	sub, super = Repr(sub), Repr(super)
	if sub == nil || super == nil {
		return false
	}
	if sub.Kind == TyError || super.Kind == TyError {
		return true
	}
	if sub == super || sub.Kind == TyBot || super.Kind == TyTop {
		return true
	}
	if st.fuel <= 0 {
		return false
	}
	key := pair{sub, super}
	if _, ok := st.assume[key]; ok {
		return true
	}
	st.fuel--
	st.assume[key] = struct{}{}
	ok := c.sub1(sub, super, st)
	delete(st.assume, key)
	return ok
}

func (c *Context) sub1(sub, super *Type, st *subState) bool {
	// Open path selections in the direction that preserves soundness.
	// A selection on the left contributes its upper bound. A selection on the
	// right contributes its lower bound.
	if sub.Kind == TySelect {
		if up, ok := c.boundOf(sub, true, st); ok && up != sub {
			return c.sub(up, super, st)
		}
	}
	if super.Kind == TySelect {
		if lo, ok := c.boundOf(super, false, st); ok && lo != super {
			return c.sub(sub, lo, st)
		}
	}

	if sub.Kind == TyVar {
		if sub == super {
			return true
		}
		// A rigid variable is a subtype of anything its upper bound is.
		if sub.Hi != nil && sub.Hi != sub {
			return c.sub(sub.Hi, super, st)
		}
		return false
	}
	if super.Kind == TyVar {
		if super.Lo != nil && super.Lo != super {
			return c.sub(sub, super.Lo, st)
		}
		return false
	}

	switch super.Kind {
	case TyInter:
		return c.sub(sub, super.L, st) && c.sub(sub, super.R, st)
	case TyUnion:
		return c.sub(sub, super.L, st) || c.sub(sub, super.R, st)
	case TyFun:
		if sub.Kind == TyFun {
			return c.subFun(sub, super, st)
		}
	}

	switch sub.Kind {
	case TyUnion:
		return c.sub(sub.L, super, st) && c.sub(sub.R, super, st)
	case TyInter:
		return c.sub(sub.L, super, st) || c.sub(sub.R, super, st)
	case TyFun:
		return false
	case TyLiteral:
		return c.subLiteral(sub, super, st)
	case TySingleton:
		if super.Kind == TySingleton && sub.Path.Equal(super.Path) {
			return true
		}
		wide := c.widenSingleton(sub)
		if wide != nil && wide != sub {
			return c.sub(wide, super, st)
		}
		return false
	case TyNamed:
		return c.subNamed(sub, super, st)
	case TyRefine:
		if c.sub(sub.Parent, super, st) {
			return true
		}
		return false
	default:
		return false
	}
}

func (c *Context) subFun(sub, super *Type, st *subState) bool {
	if len(sub.Params) != len(super.Params) {
		return false
	}
	for i := range sub.Params {
		// Parameters are contravariant.
		if !c.sub(super.Params[i], sub.Params[i], st) {
			return false
		}
	}
	return c.sub(sub.Res, super.Res, st)
}

func (c *Context) subLiteral(sub, super *Type, st *subState) bool {
	if super.Kind == TyLiteral && sub.Lit == super.Lit {
		switch sub.Lit {
		case LitInt:
			return sub.Int == super.Int
		case LitBool:
			return sub.Bool == super.Bool
		case LitString:
			return sub.String == super.String
		case LitChar:
			return sub.Char == super.Char
		case LitUnit, LitNull:
			return true
		}
	}
	switch sub.Lit {
	case LitInt:
		return c.sub(c.IntType(), super, st)
	case LitFloat:
		return c.sub(c.DoubleType(), super, st)
	case LitBool:
		return c.sub(c.BooleanType(), super, st)
	case LitString:
		return c.sub(c.StringType(), super, st)
	case LitChar:
		return c.sub(c.CharType(), super, st)
	case LitUnit:
		return c.sub(c.UnitType(), super, st)
	case LitNull:
		return c.sub(c.NullType(), super, st)
	default:
		return false
	}
}

func (c *Context) subNamed(sub, super *Type, st *subState) bool {
	if sub.Sym == c.Null && c.isAnyRef(super) {
		return true
	}
	if super.Kind == TyNamed && sub.Sym == super.Sym {
		return c.subArgs(sub.Sym, sub.Args, super.Args, st)
	}
	if super.Kind == TyNamed || super.Kind == TyTop {
		for _, p := range sub.Sym.Parents {
			parentSym := c.classSymbol(p)
			if parentSym == sub.Sym {
				continue
			}
			if c.sub(c.substClassArgs(p, sub.Sym, sub.Args), super, st) {
				return true
			}
		}
	}
	if super.Kind == TyRefine {
		return c.sub(sub, super.Parent, st)
	}
	// Null is a subtype of every AnyRef class, and of type parameters bounded
	// by one, but not of AnyVal classes. The Null symbol's parents encode this
	// when they were declared; the clause above handles it. Nothing is TyBot
	// only when it is the bottom type. The Nothing class, if distinct, is a
	// parent of nobody and a subtype via its own parents being empty and a
	// special case: the Nothing symbol is stored and compared as Bot in the
	// universe. See InitUniverse.
	return false
}

func (c *Context) subArgs(sym *Symbol, subArgs, superArgs []*Type, st *subState) bool {
	params := classParams(sym)
	if len(subArgs) != len(superArgs) || len(subArgs) != len(params) {
		// Monomorphic classes, or a mismatch, are equal only when both are empty.
		return len(subArgs) == 0 && len(superArgs) == 0
	}
	for i := range subArgs {
		switch params[i].Variance {
		case Covariant:
			if !c.sub(subArgs[i], superArgs[i], st) {
				return false
			}
		case Contravariant:
			if !c.sub(superArgs[i], subArgs[i], st) {
				return false
			}
		default:
			if !c.sub(subArgs[i], superArgs[i], st) || !c.sub(superArgs[i], subArgs[i], st) {
				return false
			}
		}
	}
	return true
}

func classParams(sym *Symbol) []*Symbol {
	if sym == nil {
		return nil
	}
	var out []*Symbol
	for _, m := range sym.Members {
		if m.Kind == SymTypeParam {
			out = append(out, m)
		}
	}
	return out
}

// substClassArgs replaces the class's type parameters inside parent with args.
func (c *Context) substClassArgs(parent *Type, class *Symbol, args []*Type) *Type {
	params := classParams(class)
	if len(params) == 0 || len(params) != len(args) {
		return parent
	}
	m := map[*Type]*Type{}
	for i, p := range params {
		if p.Skolem != nil {
			m[p.Skolem] = args[i]
		}
	}
	if len(m) == 0 {
		return parent
	}
	return c.Subst(parent, m)
}

// boundOf exposes a path selection to its upper bound (upper true) or lower bound.
func (c *Context) boundOf(sel *Type, upper bool, st *subState) (*Type, bool) {
	if sel == nil || sel.Kind != TySelect {
		return sel, false
	}
	if _, ok := st.opened[sel]; ok {
		return sel, false
	}
	st.opened[sel] = struct{}{}
	defer delete(st.opened, sel)
	sym, prefixType := c.findTypeMember(sel.Path, sel.Sel)
	if sym == nil {
		return sel, false
	}
	var b *Type
	if upper {
		b = sym.Result
		if b == nil && sym.Skolem != nil {
			b = sym.Skolem.Hi
		}
	} else {
		b = sym.Type
		if b == nil && sym.Skolem != nil {
			b = sym.Skolem.Lo
		}
	}
	if b == nil {
		if upper {
			b = c.Top
		} else {
			b = c.Bot
		}
	}
	if prefixType != nil {
		b = c.substThis(b, sel.Path, prefixType)
	}
	return b, true
}

// widenSingleton returns the underlying type of a singleton path.
func (c *Context) widenSingleton(t *Type) *Type {
	if t == nil || t.Kind != TySingleton || t.Path == nil || t.Path.Head == nil {
		return t
	}
	head := t.Path.Head
	if head.Kind == SymTerm || head.Kind == SymMethod {
		return head.Type
	}
	if head.IsClassLike() {
		// `this` inside a class. The synthetic this symbol's Type is the class type.
		return head.Type
	}
	return head.Type
}

// Lub is the least upper bound. When neither side wins, the bound is a union.
func (c *Context) Lub(a, b *Type) *Type {
	a, b = Repr(a), Repr(b)
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Kind == TyError || b.Kind == TyError {
		return c.Error
	}
	if c.Subtype(a, b) {
		return b
	}
	if c.Subtype(b, a) {
		return a
	}
	return c.Union(a, b)
}

// Glb is the greatest lower bound. When neither side wins, the bound is an intersection.
func (c *Context) Glb(a, b *Type) *Type {
	a, b = Repr(a), Repr(b)
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Kind == TyError || b.Kind == TyError {
		return c.Error
	}
	if c.Subtype(a, b) {
		return a
	}
	if c.Subtype(b, a) {
		return b
	}
	return c.Inter(a, b)
}

// Constrain records found <: expected, binding inference variables.
// Rigid failure returns false and does not mutate skolems.
func (c *Context) Constrain(found, expected *Type) bool {
	found, expected = Repr(found), Repr(expected)
	if found == nil || expected == nil {
		return false
	}
	if found.Kind == TyError || expected.Kind == TyError {
		return true
	}
	if found == expected {
		return true
	}
	if expected.Kind == TyVar && !expected.Skolem {
		expected.Lo = c.Lub(expected.Lo, found)
		return c.boundsConsistent(expected)
	}
	if found.Kind == TyVar && !found.Skolem {
		found.Hi = c.Glb(found.Hi, expected)
		return c.boundsConsistent(found)
	}
	if found.Kind == TyFun && expected.Kind == TyFun && len(found.Params) == len(expected.Params) {
		for i := range found.Params {
			if !c.Constrain(expected.Params[i], found.Params[i]) {
				return false
			}
		}
		return c.Constrain(found.Res, expected.Res)
	}
	if found.Kind == TyNamed && expected.Kind == TyNamed && found.Sym == expected.Sym {
		params := classParams(found.Sym)
		if len(found.Args) == len(expected.Args) && len(found.Args) == len(params) {
			ok := true
			for i := range found.Args {
				switch params[i].Variance {
				case Covariant:
					ok = c.Constrain(found.Args[i], expected.Args[i])
				case Contravariant:
					ok = c.Constrain(expected.Args[i], found.Args[i])
				default:
					ok = c.Constrain(found.Args[i], expected.Args[i]) && c.Constrain(expected.Args[i], found.Args[i])
				}
				if !ok {
					return false
				}
			}
			return true
		}
	}
	return c.Subtype(found, expected)
}

func (c *Context) boundsConsistent(v *Type) bool {
	if v == nil || v.Kind != TyVar {
		return true
	}
	lo, hi := Repr(v.Lo), Repr(v.Hi)
	if lo == nil || hi == nil || lo.Kind == TyBot || hi.Kind == TyTop {
		return true
	}
	// Compare bounds without recursing through this variable.
	savedLo, savedHi := v.Lo, v.Hi
	v.Lo, v.Hi = c.Bot, c.Top
	ok := c.Subtype(lo, hi)
	v.Lo, v.Hi = savedLo, savedHi
	return ok
}

// Instantiate replaces inference variables with their best known bound.
// A variable whose only bound is Any (or Nothing on the lower side alone)
// becomes the error type when require is set, so an unconstrained type
// parameter cannot leak into a typed program.
func (c *Context) Instantiate(t *Type, require bool) *Type {
	seen := map[*Type]*Type{}
	var rec func(*Type) *Type
	rec = func(t *Type) *Type {
		t = Repr(t)
		if t == nil {
			return nil
		}
		if prev, ok := seen[t]; ok {
			return prev
		}
		if t.Kind == TyVar && !t.Skolem {
			var pick *Type
			if t.Lo != nil && Repr(t.Lo).Kind != TyBot {
				pick = Repr(t.Lo)
			} else if t.Hi != nil && Repr(t.Hi).Kind != TyTop {
				pick = Repr(t.Hi)
			} else if require {
				pick = c.Error
			} else {
				pick = t
			}
			seen[t] = pick
			if pick.Kind == TyVar && !pick.Skolem && pick != t {
				pick = rec(pick)
				seen[t] = pick
			}
			return pick
		}
		switch t.Kind {
		case TyFun:
			params := make([]*Type, len(t.Params))
			for i, p := range t.Params {
				params[i] = rec(p)
			}
			return c.Fun(params, rec(t.Res))
		case TyNamed:
			if len(t.Args) == 0 {
				return t
			}
			args := make([]*Type, len(t.Args))
			changed := false
			for i, a := range t.Args {
				args[i] = rec(a)
				if args[i] != a {
					changed = true
				}
			}
			if !changed {
				return t
			}
			return c.Named(t.Sym, args)
		case TyUnion:
			return c.Union(rec(t.L), rec(t.R))
		case TyInter:
			return c.Inter(rec(t.L), rec(t.R))
		default:
			return t
		}
	}
	return rec(t)
}
