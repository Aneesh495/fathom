// Package types is the semantic model of Fathom.
//
// Surface syntax is elaborated into symbols and interned types. Subtyping is a
// fuel-bounded coinductive relation in the style of kernel DOT: path selections
// open to their bounds, unions and intersections are lattice operations, and
// class parents are a Scala linearization. A "yes" answer is sound. Running
// out of fuel is a conservative "no", never a false "yes".
package types

import "github.com/Aneesh495/fathom/internal/span"

// SymKind classifies symbols.
type SymKind int

const (
	SymPackage SymKind = iota
	SymClass
	SymTrait
	SymObject
	SymMethod
	SymTerm
	SymType
	SymTypeParam
)

func (k SymKind) String() string {
	switch k {
	case SymPackage:
		return "package"
	case SymClass:
		return "class"
	case SymTrait:
		return "trait"
	case SymObject:
		return "object"
	case SymMethod:
		return "method"
	case SymTerm:
		return "value"
	case SymType:
		return "type"
	case SymTypeParam:
		return "type parameter"
	default:
		return "symbol"
	}
}

// Flags are boolean properties of a symbol.
type Flags uint32

const (
	FlagAbstract Flags = 1 << iota
	FlagFinal
	FlagSealed
	FlagOverride
	FlagMutable
	FlagBuiltin
	FlagSynthetic
	FlagOpaque
	FlagCase
	FlagModule
	FlagGiven
	FlagExtension
	FlagParam
	FlagStable
	FlagMain
	FlagCovariant
	FlagContravariant
	FlagPrivate
	FlagByName
)

// Builtin identifies a primitive operation the compiler lowers directly.
type Builtin int

const (
	BuiltinNone Builtin = iota
	BuiltinAdd
	BuiltinSub
	BuiltinMul
	BuiltinDiv
	BuiltinMod
	BuiltinNeg
	BuiltinEq
	BuiltinNe
	BuiltinLt
	BuiltinLe
	BuiltinGt
	BuiltinGe
	BuiltinAnd
	BuiltinOr
	BuiltinNot
	BuiltinBitAnd
	BuiltinBitOr
	BuiltinBitXor
	BuiltinBitNot
	BuiltinShl
	BuiltinShr
	BuiltinConcat
	BuiltinToString
	BuiltinPrint
	BuiltinPrintln
	BuiltinAssert
	BuiltinLength
	BuiltinCharAt
	BuiltinSubstring
)

// Symbol is a named entity in the program.
// Members keeps declaration order. The map is an index into Members.
type Symbol struct {
	Name    string
	Kind    SymKind
	Flags   Flags
	Span    span.Span
	Owner   *Symbol
	Type    *Type
	Parents []*Type
	Members []*Symbol
	index   map[string][]*Symbol
	Scope   *Scope
	Lin     []*Symbol

	// Signature of a method. TParams are the method's own type parameters.
	TParams []*Symbol
	Lists   []ParamList
	Result  *Type

	// Type parameter variance and bounds. Bounds live on Skolem as well.
	Variance Variance
	Skolem   *Type

	// ID is a process-local identity used as an intern key. Index is the
	// field, local, or parameter slot assigned during lowering.
	ID      int
	Index   int
	Builtin Builtin
	// Companion links a class and its module, in both directions.
	Companion *Symbol
	// AST is the declaring node, stored as any to keep this package free of
	// a dependency on the parser's node set beyond span. The checker fills it.
	AST any
}

// Variance of a type parameter.
type Variance int

const (
	Invariant Variance = iota
	Covariant
	Contravariant
)

// ParamList is one parameter clause of a method.
type ParamList struct {
	Using  bool
	Params []*Symbol
}

// Has reports a flag.
func (s *Symbol) Has(f Flags) bool {
	return s != nil && s.Flags&f != 0
}

// IsClassLike reports classes, traits, and objects, which all have linearizations.
func (s *Symbol) IsClassLike() bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case SymClass, SymTrait, SymObject:
		return true
	default:
		return false
	}
}

// IsType reports symbols that may appear in type position.
func (s *Symbol) IsType() bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case SymClass, SymTrait, SymObject, SymType, SymTypeParam:
		return true
	default:
		return false
	}
}

// AddMember records m under its name. Overloads append.
func (s *Symbol) AddMember(m *Symbol) {
	if s == nil || m == nil {
		return
	}
	if s.index == nil {
		s.index = map[string][]*Symbol{}
	}
	s.Members = append(s.Members, m)
	s.index[m.Name] = append(s.index[m.Name], m)
	m.Owner = s
}

// Get returns the first member with this name, or nil.
func (s *Symbol) Get(name string) *Symbol {
	all := s.GetAll(name)
	if len(all) == 0 {
		return nil
	}
	return all[0]
}

// GetAll returns every overload of name in declaration order.
func (s *Symbol) GetAll(name string) []*Symbol {
	if s == nil || s.index == nil {
		return nil
	}
	return s.index[name]
}

// FullName is the dotted owner path, skipping synthetic owners.
func (s *Symbol) FullName() string {
	if s == nil {
		return ""
	}
	if s.Owner == nil || s.Owner.Kind == SymPackage && s.Owner.Name == "" {
		return s.Name
	}
	if s.Owner.Name == "" {
		return s.Name
	}
	return s.Owner.FullName() + "." + s.Name
}

// Scope is a lexical environment. Children shadow parents.
type Scope struct {
	Parent *Scope
	Owner  *Symbol
	table  map[string][]*Symbol
	order  []*Symbol
}

// NewScope returns a child of parent.
func NewScope(parent *Scope, owner *Symbol) *Scope {
	return &Scope{Parent: parent, Owner: owner, table: map[string][]*Symbol{}}
}

// Bind adds a symbol. It does not replace outer bindings; lookup finds this one first.
func (s *Scope) Bind(sym *Symbol) {
	if s == nil || sym == nil {
		return
	}
	if s.table == nil {
		s.table = map[string][]*Symbol{}
	}
	s.table[sym.Name] = append(s.table[sym.Name], sym)
	s.order = append(s.order, sym)
}

// Lookup walks outward. The nearest scope's bindings are returned in order.
func (s *Scope) Lookup(name string) []*Symbol {
	for sc := s; sc != nil; sc = sc.Parent {
		if hits := sc.table[name]; len(hits) > 0 {
			out := make([]*Symbol, len(hits))
			copy(out, hits)
			return out
		}
	}
	return nil
}

// Local reports whether name is bound in this scope and not a parent.
func (s *Scope) Local(name string) []*Symbol {
	if s == nil {
		return nil
	}
	return s.table[name]
}

// Names returns the names bound in this scope, in insertion order, without duplicates.
func (s *Scope) Names() []string {
	if s == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, sym := range s.order {
		if sym == nil || seen[sym.Name] {
			continue
		}
		seen[sym.Name] = true
		out = append(out, sym.Name)
	}
	return out
}
