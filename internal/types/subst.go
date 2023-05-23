package types

// Subst replaces types in m. Keys are usually skolems of type parameters.
// The result is interned, so equal substitutions share pointers.
func (c *Context) Subst(t *Type, m map[*Type]*Type) *Type {
	if t == nil || len(m) == 0 {
		return t
	}
	seen := map[*Type]*Type{}
	var rec func(*Type) *Type
	rec = func(t *Type) *Type {
		t = Repr(t)
		if t == nil {
			return nil
		}
		if r, ok := m[t]; ok {
			return r
		}
		if r, ok := seen[t]; ok {
			return r
		}
		switch t.Kind {
		case TyFun:
			params := make([]*Type, len(t.Params))
			for i, p := range t.Params {
				params[i] = rec(p)
			}
			out := c.Fun(params, rec(t.Res))
			seen[t] = out
			return out
		case TyNamed:
			if len(t.Args) == 0 {
				return t
			}
			args := make([]*Type, len(t.Args))
			changed := false
			for i, a := range t.Args {
				args[i] = rec(a)
				if args[i] != Repr(a) {
					changed = true
				}
			}
			if !changed {
				return t
			}
			out := c.Named(t.Sym, args)
			seen[t] = out
			return out
		case TyUnion:
			out := c.Union(rec(t.L), rec(t.R))
			seen[t] = out
			return out
		case TyInter:
			out := c.Inter(rec(t.L), rec(t.R))
			seen[t] = out
			return out
		case TySingleton:
			return t
		case TySelect:
			return t
		default:
			return t
		}
	}
	return rec(t)
}

// substThis replaces occurrences of the class's `this` path inside t with path.
// prefixType is the widened type of that path, used when a selection's bound
// still says `this`.
func (c *Context) substThis(t *Type, path *Path, prefixType *Type) *Type {
	if t == nil || path == nil {
		return t
	}
	thisPath := thisOf(prefixType)
	seen := map[*Type]*Type{}
	var rec func(*Type) *Type
	rec = func(t *Type) *Type {
		t = Repr(t)
		if t == nil {
			return nil
		}
		if r, ok := seen[t]; ok {
			return r
		}
		switch t.Kind {
		case TySelect:
			if t.Path != nil && thisPath != nil && t.Path.Equal(thisPath) {
				out := c.Select(path, t.Sel)
				seen[t] = out
				return out
			}
			return t
		case TySingleton:
			if t.Path != nil && thisPath != nil && t.Path.Equal(thisPath) {
				out := c.Singleton(path)
				seen[t] = out
				return out
			}
			return t
		case TyFun:
			params := make([]*Type, len(t.Params))
			for i, p := range t.Params {
				params[i] = rec(p)
			}
			out := c.Fun(params, rec(t.Res))
			seen[t] = out
			return out
		case TyUnion:
			out := c.Union(rec(t.L), rec(t.R))
			seen[t] = out
			return out
		case TyInter:
			out := c.Inter(rec(t.L), rec(t.R))
			seen[t] = out
			return out
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
			out := c.Named(t.Sym, args)
			seen[t] = out
			return out
		default:
			return t
		}
	}
	return rec(t)
}

func thisOf(t *Type) *Path {
	t = Repr(t)
	if t == nil {
		return nil
	}
	if t.Kind == TyNamed && t.Sym != nil {
		if th := t.Sym.Get("this"); th != nil {
			return &Path{Head: th}
		}
	}
	if t.Kind == TySingleton {
		return t.Path
	}
	return nil
}
