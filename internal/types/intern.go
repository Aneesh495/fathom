package types

import (
	"strconv"
	"strings"
)

// Context owns the intern table, the universe, and fuel for subtyping.
type Context struct {
	Fuel int

	Top   *Type
	Bot   *Type
	Error *Type

	Any     *Symbol
	AnyVal  *Symbol
	AnyRef  *Symbol
	Int     *Symbol
	Double  *Symbol
	Boolean *Symbol
	Char    *Symbol
	String  *Symbol
	Unit    *Symbol
	Nothing *Symbol
	Null    *Symbol
	Tuple2  *Symbol
	Tuple3  *Symbol
	Tuple4  *Symbol

	intT     *Type
	doubleT  *Type
	booleanT *Type
	charT    *Type
	stringT  *Type
	unitT    *Type
	nullT    *Type

	nextID   int
	nextVar  int
	nextSym  int
	table    map[string]*Type
	visiting map[*Symbol]bool
}

// NewSymbol allocates a symbol with a stable identity for interning.
func (c *Context) NewSymbol(name string, kind SymKind) *Symbol {
	c.nextSym++
	return &Symbol{Name: name, Kind: kind, ID: c.nextSym}
}

// NewContext builds an empty intern table. Call InitUniverse before checking.
func NewContext() *Context {
	c := &Context{
		Fuel:  96,
		table: map[string]*Type{},
	}
	c.Top = c.mk(TyTop, "top")
	c.Bot = c.mk(TyBot, "bot")
	c.Error = c.mk(TyError, "error")
	return c
}

func (c *Context) mk(k Kind, key string) *Type {
	if prev, ok := c.table[key]; ok {
		return prev
	}
	c.nextID++
	t := &Type{Kind: k, ID: c.nextID}
	c.table[key] = t
	return t
}

// Named returns a class type applied to args. Nil args mean a monomorphic class.
func (c *Context) Named(sym *Symbol, args []*Type) *Type {
	if sym == nil {
		return c.Error
	}
	var b strings.Builder
	b.WriteString("named:")
	b.WriteString(strconv.Itoa(symbolID(sym)))
	for _, a := range args {
		b.WriteByte(',')
		b.WriteString(idOf(a))
	}
	t := c.mk(TyNamed, b.String())
	t.Sym = sym
	t.Args = args
	return t
}

// Fun returns a function type. Parameter names are not part of the type.
func (c *Context) Fun(params []*Type, res *Type) *Type {
	var b strings.Builder
	b.WriteString("fun:")
	for i, p := range params {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(idOf(p))
	}
	b.WriteByte('>')
	b.WriteString(idOf(res))
	t := c.mk(TyFun, b.String())
	t.Params = params
	t.Res = res
	return t
}

// Union returns a flattened, ordered union. Bot is the identity. Top absorbs.
func (c *Context) Union(parts ...*Type) *Type {
	flat := flatten(TyUnion, parts)
	flat = uniqueTypes(flat)
	if len(flat) == 0 {
		return c.Bot
	}
	var kept []*Type
	for _, p := range flat {
		p = Repr(p)
		if p == nil || p.Kind == TyBot {
			continue
		}
		if p.Kind == TyTop || p.Kind == TyError {
			return p
		}
		kept = append(kept, p)
	}
	kept = uniqueTypes(kept)
	if len(kept) == 0 {
		return c.Bot
	}
	if len(kept) == 1 {
		return kept[0]
	}
	sortTypes(kept)
	return c.nary(TyUnion, kept)
}

// Inter returns a flattened intersection. Top is the identity. Bot absorbs.
func (c *Context) Inter(parts ...*Type) *Type {
	flat := flatten(TyInter, parts)
	var kept []*Type
	for _, p := range flat {
		p = Repr(p)
		if p == nil || p.Kind == TyTop {
			continue
		}
		if p.Kind == TyBot || p.Kind == TyError {
			return p
		}
		kept = append(kept, p)
	}
	kept = uniqueTypes(kept)
	if len(kept) == 0 {
		return c.Top
	}
	if len(kept) == 1 {
		return kept[0]
	}
	sortTypes(kept)
	return c.nary(TyInter, kept)
}

func (c *Context) nary(k Kind, parts []*Type) *Type {
	// Build a right-nested binary tree. Sorting makes it canonical.
	cur := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		var b strings.Builder
		if k == TyUnion {
			b.WriteString("union:")
		} else {
			b.WriteString("inter:")
		}
		b.WriteString(idOf(parts[i]))
		b.WriteByte('|')
		b.WriteString(idOf(cur))
		t := c.mk(k, b.String())
		t.L = parts[i]
		t.R = cur
		cur = t
	}
	return cur
}

// Singleton returns the singleton type of a stable path.
func (c *Context) Singleton(path *Path) *Type {
	if path == nil || path.Head == nil {
		return c.Error
	}
	key := "sing:" + path.String() + "#" + strconv.Itoa(symbolID(path.Head))
	t := c.mk(TySingleton, key)
	t.Path = path.Clone()
	return t
}

// Select returns the path-dependent type path.sel.
func (c *Context) Select(path *Path, sel string) *Type {
	if path == nil || path.Head == nil || sel == "" {
		return c.Error
	}
	key := "sel:" + path.String() + "#" + strconv.Itoa(symbolID(path.Head)) + "." + sel
	t := c.mk(TySelect, key)
	t.Path = path.Clone()
	t.Sel = sel
	return t
}

// LiteralInt returns the singleton type of an integer, which is a subtype of Int.
func (c *Context) LiteralInt(v int64) *Type {
	key := "lit:i:" + strconv.FormatInt(v, 10)
	t := c.mk(TyLiteral, key)
	t.Lit = LitInt
	t.Int = v
	return t
}

// LiteralBool returns the singleton type of a boolean.
func (c *Context) LiteralBool(v bool) *Type {
	key := "lit:b:0"
	if v {
		key = "lit:b:1"
	}
	t := c.mk(TyLiteral, key)
	t.Lit = LitBool
	t.Bool = v
	return t
}

// LiteralString returns the singleton type of a string.
func (c *Context) LiteralString(v string) *Type {
	key := "lit:s:" + v
	t := c.mk(TyLiteral, key)
	t.Lit = LitString
	t.String = v
	return t
}

// LiteralChar returns the singleton type of a character.
func (c *Context) LiteralChar(v rune) *Type {
	key := "lit:c:" + strconv.Itoa(int(v))
	t := c.mk(TyLiteral, key)
	t.Lit = LitChar
	t.Char = v
	return t
}

// Fresh returns an inference variable. It is not interned.
func (c *Context) Fresh(name string, level int) *Type {
	c.nextVar++
	return &Type{
		Kind:  TyVar,
		Name:  name,
		VarID: c.nextVar,
		Level: level,
		Lo:    c.Bot,
		Hi:    c.Top,
	}
}

// Skolem returns a rigid type variable with the given bounds.
func (c *Context) Skolem(name string, lo, hi *Type) *Type {
	if lo == nil {
		lo = c.Bot
	}
	if hi == nil {
		hi = c.Top
	}
	c.nextVar++
	return &Type{
		Kind:   TyVar,
		Name:   name,
		VarID:  c.nextVar,
		Skolem: true,
		Lo:     lo,
		Hi:     hi,
	}
}

// IntType is the type of 64-bit integers.
func (c *Context) IntType() *Type { return c.intT }

// DoubleType is the type of IEEE-754 binary64 numbers.
func (c *Context) DoubleType() *Type { return c.doubleT }

// BooleanType is the type of booleans.
func (c *Context) BooleanType() *Type { return c.booleanT }

// CharType is the type of Unicode code points.
func (c *Context) CharType() *Type { return c.charT }

// StringType is the type of strings.
func (c *Context) StringType() *Type { return c.stringT }

// UnitType is the type of the unit value.
func (c *Context) UnitType() *Type { return c.unitT }

// NullType is the type of null.
func (c *Context) NullType() *Type { return c.nullT }

func idOf(t *Type) string {
	t = Repr(t)
	if t == nil {
		return "nil"
	}
	if t.Kind == TyVar {
		return "v" + strconv.Itoa(t.VarID)
	}
	return strconv.Itoa(t.ID)
}

func symbolID(s *Symbol) int {
	if s == nil {
		return 0
	}
	if s.ID > 0 {
		return s.ID
	}
	return nameHash(s)
}

func nameHash(s *Symbol) int {
	h := int(s.Kind) + 1
	for _, r := range s.FullName() {
		h = h*131 + int(r)
	}
	if h < 0 {
		h = -h
	}
	if h == 0 {
		h = 1
	}
	return h
}

func flatten(k Kind, parts []*Type) []*Type {
	var out []*Type
	var walk func(*Type)
	walk = func(t *Type) {
		t = Repr(t)
		if t == nil {
			return
		}
		if t.Kind == k {
			walk(t.L)
			walk(t.R)
			return
		}
		out = append(out, t)
	}
	for _, p := range parts {
		walk(p)
	}
	return out
}

func uniqueTypes(in []*Type) []*Type {
	seen := map[*Type]bool{}
	var out []*Type
	for _, t := range in {
		t = Repr(t)
		if t == nil || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func sortTypes(in []*Type) {
	// Insertion sort keeps the implementation obvious and avoids importing sort
	// for a handful of union arms.
	for i := 1; i < len(in); i++ {
		j := i
		for j > 0 && typeLess(in[j], in[j-1]) {
			in[j], in[j-1] = in[j-1], in[j]
			j--
		}
	}
}

func typeLess(a, b *Type) bool {
	if a == b {
		return false
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Kind == TyVar && b.Kind == TyVar {
		return a.VarID < b.VarID
	}
	return a.ID < b.ID
}
