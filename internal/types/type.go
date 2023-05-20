package types

// Kind classifies a semantic type.
type Kind int

const (
	TyTop Kind = iota
	TyBot
	TyError
	TyNamed
	TyVar
	TyFun
	TyInter
	TyUnion
	TySingleton
	TySelect
	TyRefine
	TyLiteral
)

func (k Kind) String() string {
	switch k {
	case TyTop:
		return "Any"
	case TyBot:
		return "Nothing"
	case TyError:
		return "<error>"
	case TyNamed:
		return "named"
	case TyVar:
		return "var"
	case TyFun:
		return "function"
	case TyInter:
		return "intersection"
	case TyUnion:
		return "union"
	case TySingleton:
		return "singleton"
	case TySelect:
		return "selection"
	case TyRefine:
		return "refinement"
	case TyLiteral:
		return "literal"
	default:
		return "type"
	}
}

// LitKind is the payload of a literal singleton type.
type LitKind int

const (
	LitInt LitKind = iota
	LitFloat
	LitBool
	LitString
	LitChar
	LitUnit
	LitNull
)

// Path is a stable term path. Head is a parameter, a stable val, or the
// synthetic `this` of a class. Tail is a chain of stable field names.
type Path struct {
	Head *Symbol
	Tail []string
}

// Equal reports syntactic path equality.
func (p *Path) Equal(o *Path) bool {
	if p == nil || o == nil {
		return p == o
	}
	if p.Head != o.Head || len(p.Tail) != len(o.Tail) {
		return false
	}
	for i := range p.Tail {
		if p.Tail[i] != o.Tail[i] {
			return false
		}
	}
	return true
}

func (p *Path) String() string {
	if p == nil || p.Head == nil {
		return "<path>"
	}
	s := p.Head.Name
	if p.Head.Name == "this" || p.Head.Has(FlagSynthetic) && p.Head.Name == "" {
		s = "this"
	}
	for _, t := range p.Tail {
		s += "." + t
	}
	return s
}

// Clone returns a path that does not share the tail slice.
func (p *Path) Clone() *Path {
	if p == nil {
		return nil
	}
	cp := &Path{Head: p.Head}
	if len(p.Tail) > 0 {
		cp.Tail = append([]string(nil), p.Tail...)
	}
	return cp
}

// Member is a field, method, or type member discovered by lookup.
type Member struct {
	Sym    *Symbol
	Type   *Type
	Prefix *Path
}

// Type is an interned semantic type, except for type variables, which are
// unique pointers because inference mutates their bounds and link.
type Type struct {
	Kind Kind
	ID   int

	Sym  *Symbol
	Args []*Type

	Params []*Type
	Res    *Type

	L, R *Type

	// Variable state. Lo and Hi are also the bounds of an abstract type member
	// when Kind is TyNamed and Sym.Kind is SymType, but those bounds are read
	// from the symbol. Vars store them here.
	Name   string
	VarID  int
	Skolem bool
	Level  int
	Link   *Type
	Lo, Hi *Type

	Path *Path
	Sel  string

	// Refinement parent and locally declared members.
	Parent  *Type
	Refined []*Symbol

	Lit    LitKind
	Int    int64
	Float  float64
	Bool   bool
	String string
	Char   rune
}

// Repr follows inference links to a type that is not a bound variable.
func Repr(t *Type) *Type {
	seen := map[*Type]bool{}
	for t != nil && t.Kind == TyVar && t.Link != nil {
		if seen[t] {
			return t
		}
		seen[t] = true
		next := t.Link
		if next != nil && next.Kind == TyVar && next.Link != nil {
			t.Link = Repr(next)
			return t.Link
		}
		t = next
	}
	return t
}

// IsError reports the error type, which subtyping treats as both bounds so
// that one mistake does not flood the rest of the program with mismatches.
func (t *Type) IsError() bool {
	t = Repr(t)
	return t != nil && t.Kind == TyError
}

// Same reports pointer equality after representation.
func Same(a, b *Type) bool {
	return Repr(a) == Repr(b)
}
