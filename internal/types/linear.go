package types

// Linearize computes the Scala linearization of a class, trait, or object.
//
// For `C extends C1 with C2 with ... with Cn` the linearization is
// C followed by the merge of L(Cn), ..., L(C1). Merge picks the first head
// that does not appear in the tail of any list, which is the C3 condition.
// The result is cached on the symbol. A cycle in the parent relation yields
// a partial order rather than looping.
func (c *Context) Linearize(sym *Symbol) []*Symbol {
	if sym == nil {
		return nil
	}
	if sym.Lin != nil {
		return sym.Lin
	}
	if c.visiting == nil {
		c.visiting = map[*Symbol]bool{}
	}
	if c.visiting[sym] {
		return []*Symbol{sym}
	}
	c.visiting[sym] = true
	var seqs [][]*Symbol
	for i := len(sym.Parents) - 1; i >= 0; i-- {
		ps := c.classSymbol(sym.Parents[i])
		if ps == nil || ps == sym {
			continue
		}
		lin := c.Linearize(ps)
		cp := append([]*Symbol(nil), lin...)
		seqs = append(seqs, cp)
	}
	delete(c.visiting, sym)
	merged := mergeLin(seqs)
	// Drop a repeated head, which shows up when a parent cycle was cut.
	out := []*Symbol{sym}
	seen := map[*Symbol]bool{sym: true}
	for _, s := range merged {
		if s == nil || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sym.Lin = out
	return sym.Lin
}

// isAnyRef reports whether t is a reference type. Value classes and AnyVal
// itself are not. Null uses this to decide where it may flow.
func (c *Context) isAnyRef(t *Type) bool {
	t = Repr(t)
	if t == nil || c.AnyRef == nil {
		return false
	}
	switch t.Kind {
	case TyNamed:
		if t.Sym == c.AnyVal || t.Sym == c.Nothing {
			return false
		}
		for _, s := range c.Linearize(t.Sym) {
			if s == c.AnyVal {
				return false
			}
			if s == c.AnyRef {
				return true
			}
		}
		return false
	case TySingleton:
		return c.isAnyRef(c.widenSingleton(t))
	case TyLiteral:
		return c.isAnyRef(c.literalBase(t))
	case TyUnion:
		return c.isAnyRef(t.L) && c.isAnyRef(t.R)
	case TyInter:
		return c.isAnyRef(t.L) || c.isAnyRef(t.R)
	default:
		return false
	}
}

func (c *Context) classSymbol(t *Type) *Symbol {
	t = Repr(t)
	if t == nil {
		return nil
	}
	switch t.Kind {
	case TyTop:
		return c.Any
	case TyBot:
		return c.Nothing
	case TyNamed:
		return t.Sym
	case TySingleton:
		if t.Path != nil && t.Path.Head != nil && t.Path.Head.IsClassLike() {
			return t.Path.Head
		}
		if t.Path != nil && t.Path.Head != nil {
			return c.classSymbol(t.Path.Head.Type)
		}
	}
	return nil
}

func mergeLin(seqs [][]*Symbol) []*Symbol {
	var out []*Symbol
	guard := 0
	limit := 0
	for _, s := range seqs {
		limit += len(s)
	}
	limit += 2
	for guard < limit {
		guard++
		nonempty := false
		for _, s := range seqs {
			if len(s) > 0 {
				nonempty = true
				break
			}
		}
		if !nonempty {
			return out
		}
		var pick *Symbol
		for _, s := range seqs {
			if len(s) == 0 {
				continue
			}
			h := s[0]
			if !inAnyTail(seqs, h) {
				pick = h
				break
			}
		}
		if pick == nil {
			return out
		}
		out = append(out, pick)
		for i := range seqs {
			if len(seqs[i]) > 0 && seqs[i][0] == pick {
				seqs[i] = seqs[i][1:]
			}
		}
	}
	return out
}

func inAnyTail(seqs [][]*Symbol, h *Symbol) bool {
	for _, s := range seqs {
		for i := 1; i < len(s); i++ {
			if s[i] == h {
				return true
			}
		}
	}
	return false
}

// LookupMember finds name in the linearization of t, then in refinements.
// The returned member's type is not yet substituted for a path prefix.
func (c *Context) LookupMember(t *Type, name string) *Symbol {
	t = Repr(t)
	if t == nil || name == "" {
		return nil
	}
	switch t.Kind {
	case TyNamed:
		if t.Sym == nil {
			return nil
		}
		for _, s := range c.Linearize(t.Sym) {
			if m := s.Get(name); m != nil {
				return m
			}
		}
		return nil
	case TySingleton:
		return c.LookupMember(c.widenSingleton(t), name)
	case TyInter:
		if m := c.LookupMember(t.L, name); m != nil {
			return m
		}
		return c.LookupMember(t.R, name)
	case TyRefine:
		for _, m := range t.Refined {
			if m != nil && m.Name == name {
				return m
			}
		}
		return c.LookupMember(t.Parent, name)
	case TyUnion:
		left := c.LookupMember(t.L, name)
		right := c.LookupMember(t.R, name)
		if left != nil && right != nil {
			return left
		}
		return nil
	case TyLiteral:
		return c.LookupMember(c.literalBase(t), name)
	default:
		return nil
	}
}

func (c *Context) literalBase(t *Type) *Type {
	if t == nil {
		return nil
	}
	switch t.Lit {
	case LitInt:
		return c.IntType()
	case LitFloat:
		return c.DoubleType()
	case LitBool:
		return c.BooleanType()
	case LitString:
		return c.StringType()
	case LitChar:
		return c.CharType()
	case LitUnit:
		return c.UnitType()
	case LitNull:
		return c.NullType()
	default:
		return c.Top
	}
}

// findTypeMember resolves path.sel to an abstract or alias type member and the
// widened type of the path, which supplies `this` for substitution.
func (c *Context) findTypeMember(path *Path, sel string) (*Symbol, *Type) {
	if path == nil || path.Head == nil {
		return nil, nil
	}
	base := path.Head.Type
	base = c.followFields(base, path.Tail)
	if base == nil {
		return nil, nil
	}
	m := c.LookupMember(base, sel)
	if m == nil || (m.Kind != SymType && m.Kind != SymTypeParam) {
		return nil, nil
	}
	return m, base
}

func (c *Context) followFields(t *Type, fields []string) *Type {
	for _, name := range fields {
		m := c.LookupMember(t, name)
		if m == nil {
			return nil
		}
		t = m.Type
	}
	return t
}
