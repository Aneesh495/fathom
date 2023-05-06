// Package token defines the lexical vocabulary of Fathom.
// The set is the brace dialect of a Scala 3 kernel: keywords, punctuation,
// and the operator precedence Scala assigns by the first character of a symbol.
package token

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Aneesh495/fathom/internal/span"
)

// Kind classifies a token. Operators that are not their own keyword live in Op,
// and the spelling is Token.Lit.
type Kind uint16

const (
	EOF Kind = iota
	Illegal

	Ident
	Int
	Float
	String
	Char
	// InterpStart begins an s"..." or raw"..." string.
	// Lit is the interpolator name. The following tokens are the hole, then
	// either InterpMid (another string chunk followed by a hole) or InterpEnd.
	InterpStart
	InterpMid
	InterpEnd
	// StrPart is a literal chunk inside an interpolation. Lit is the decoded text.
	StrPart

	LParen
	RParen
	LBrack
	RBrack
	LBrace
	RBrace
	Comma
	Dot
	Colon
	Semi
	At
	Arrow     // =>
	LArrow    // <-
	Subtype   // <:
	Supertype // >:
	Assign    // =
	Question  // ?
	Hash      // # type projection

	// Keywords. The parser accepts the ones the kernel implements.
	// The rest are reserved so they cannot be used as identifiers.
	Abstract
	Case
	Catch
	Class
	Def
	Do
	Else
	End
	Enum
	Extends
	False
	Final
	Finally
	For
	Given
	If
	Implicit
	Import
	Infix
	Inline
	Lazy
	Match
	New
	Null
	Object
	Opaque
	Open
	Override
	Package
	Private
	Protected
	Return
	Sealed
	Super
	Then
	This
	Throw
	Trait
	Transparent
	True
	Try
	Type
	Using
	Val
	Var
	While
	With
	Yield
	Extension
	Derive

	// Op is a symbolic operator that is not one of the tokens above.
	// Examples: +  ::  &&  ==  +=
	Op
)

var kindNames = [...]string{
	EOF: "EOF", Illegal: "illegal character",
	Ident: "identifier", Int: "integer", Float: "floating point number",
	String: "string", Char: "character",
	InterpStart: "interpolation", InterpMid: "interpolation", InterpEnd: "interpolation",
	StrPart: "string part",
	LParen:  "(", RParen: ")", LBrack: "[", RBrack: "]",
	LBrace: "{", RBrace: "}", Comma: ",", Dot: ".", Colon: ":",
	Semi: ";", At: "@", Arrow: "=>", LArrow: "<-",
	Subtype: "<:", Supertype: ">:", Assign: "=", Question: "?",
	Hash:     "#",
	Abstract: "abstract", Case: "case", Catch: "catch", Class: "class",
	Def: "def", Do: "do", Else: "else", End: "end", Enum: "enum",
	Extends: "extends", False: "false", Final: "final", Finally: "finally",
	For: "for", Given: "given", If: "if", Implicit: "implicit", Import: "import",
	Infix: "infix", Inline: "inline", Lazy: "lazy", Match: "match", New: "new",
	Null: "null", Object: "object", Opaque: "opaque", Open: "open",
	Override: "override", Package: "package", Private: "private",
	Protected: "protected", Return: "return", Sealed: "sealed", Super: "super",
	Then: "then", This: "this", Throw: "throw", Trait: "trait",
	Transparent: "transparent", True: "true", Try: "try", Type: "type",
	Using: "using", Val: "val", Var: "var", While: "while", With: "with",
	Yield: "yield", Extension: "extension", Derive: "derive",
	Op: "operator",
}

// String returns a short display name for the kind.
func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "token"
}

// Keyword maps a bare identifier spelling to a keyword kind.
// The boolean is false when lit is an ordinary identifier.
func Keyword(lit string) (Kind, bool) {
	k, ok := keywords[lit]
	return k, ok
}

var keywords = map[string]Kind{
	"abstract": Abstract, "case": Case, "catch": Catch, "class": Class,
	"def": Def, "do": Do, "else": Else, "end": End, "enum": Enum,
	"extends": Extends, "false": False, "final": Final, "finally": Finally,
	"for": For, "given": Given, "if": If, "implicit": Implicit, "import": Import,
	"infix": Infix, "inline": Inline, "lazy": Lazy, "match": Match, "new": New,
	"null": Null, "object": Object, "opaque": Opaque, "open": Open,
	"override": Override, "package": Package, "private": Private,
	"protected": Protected, "return": Return, "sealed": Sealed, "super": Super,
	"then": Then, "this": This, "throw": Throw, "trait": Trait,
	"transparent": Transparent, "true": True, "try": Try, "type": Type,
	"using": Using, "val": Val, "var": Var, "while": While, "with": With,
	"yield": Yield, "extension": Extension, "derive": Derive,
}

// Token is one lexical element. Lit is set for identifiers, literals, and operators.
// Inserted is true for semicolons invented by newline handling.
type Token struct {
	Kind Kind
	Lit  string
	Span span.Span
	// Inserted is true for semicolons invented from a newline.
	Inserted bool
	// Newline is true when a line break outside parentheses or brackets
	// preceded this token. Semi insertion consumes the flag; the parser
	// does not look at it.
	Newline bool
}

func (t Token) String() string {
	if t.Lit != "" && (t.Kind == Ident || t.Kind == Op || t.Kind == Int || t.Kind == Float || t.Kind == String || t.Kind == Char || t.Kind == StrPart || t.Kind == Illegal) {
		return t.Lit
	}
	return t.Kind.String()
}

// IsIdent reports whether the token can be read as a name.
// Soft keywords that we still lex as keywords are not identifiers.
func (t Token) IsIdent() bool { return t.Kind == Ident }

// CanEndStmt reports whether a newline after this token may separate statements.
func (k Kind) CanEndStmt() bool {
	switch k {
	case Ident, Int, Float, String, Char, InterpEnd, RParen, RBrack, RBrace,
		True, False, Null, This, Super, Return, Throw:
		return true
	default:
		return false
	}
}

// ContinuesLine reports whether a token at the start of a line means the
// previous line was not finished. This is the newline rule from Scala's
// lexical syntax, trimmed to the constructs Fathom parses.
func (k Kind) ContinuesLine() bool {
	switch k {
	case Dot, Op, Assign, Arrow, Colon, Subtype, Supertype, Comma,
		LParen, LBrack, Else, Yield, Catch, Finally, With, Match, Extends, Do, Then:
		return true
	default:
		return false
	}
}

// Prec is an operator precedence. Higher binds tighter.
// Numbers follow the Scala first-character rule so that `::` and `+` behave
// the way Scala programmers expect, including right associativity of operators
// whose spelling ends in `:`.
type Prec int

const (
	PrecNone Prec = iota
	PrecAssign
	PrecOrType // unused at expr level; type syntax has its own ladder
	PrecLetter // named operators such as eq
	PrecPipe
	PrecCaret
	PrecAmp
	PrecRel
	PrecCons // colon operators, :
	PrecAdd
	PrecMul
	PrecPrefix
	PrecPost
)

// OpPrec returns the precedence and associativity of a symbolic operator.
// right is true when the operator ends with ':' (Scala's cons rule), except
// for the comparison spellings `<:` and `>:` which are types, not terms.
func OpPrec(lit string) (p Prec, right bool) {
	if lit == "" {
		return PrecNone, false
	}
	switch lit[0] {
	case '*', '/', '%':
		p = PrecMul
	case '+', '-':
		p = PrecAdd
	case ':':
		p = PrecCons
	case '=', '!':
		p = PrecRel
	case '<', '>':
		p = PrecRel
	case '&':
		p = PrecAmp
	case '^':
		p = PrecCaret
	case '|':
		p = PrecPipe
	default:
		if lit[0] >= 'A' && lit[0] <= 'Z' || lit[0] >= 'a' && lit[0] <= 'z' || lit[0] == '_' {
			p = PrecLetter
		} else {
			p = PrecLetter
		}
	}
	if lit[len(lit)-1] == ':' && lit != "<:" && lit != ">:" {
		right = true
	}
	return p, right
}

// IsAssignmentOp reports compound assignment such as += or &&=.
// Plain '=' is Assign, not an operator, so it is handled by the parser directly.
func IsAssignmentOp(lit string) bool {
	if len(lit) < 2 || lit[len(lit)-1] != '=' {
		return false
	}
	switch lit {
	case "==", "!=", "<=", ">=", "===", "eq=":
		return false
	}
	// `<=` already excluded. `<<=` is a compound assignment.
	return true
}

// AssignBase strips a trailing '=' from a compound assignment operator.
func AssignBase(lit string) string {
	if IsAssignmentOp(lit) {
		return lit[:len(lit)-1]
	}
	return lit
}

// ParseInt parses an integer literal. Underscores are ignored.
// The base is 16 for an 0x prefix, 2 for an 0b prefix, and 10 otherwise.
// The value must fit in a signed 64-bit integer.
func ParseInt(lit string) (int64, error) {
	s := stripUnderscore(lit)
	base := 10
	switch {
	case len(s) > 2 && (s[0] == '0' && (s[1] == 'x' || s[1] == 'X')):
		base = 16
		s = s[2:]
	case len(s) > 2 && (s[0] == '0' && (s[1] == 'b' || s[1] == 'B')):
		base = 2
		s = s[2:]
	}
	if s == "" || s == "+" || s == "-" {
		return 0, fmt.Errorf("empty integer")
	}
	return strconv.ParseInt(s, base, 64)
}

// ParseFloat parses a decimal literal. Underscores are ignored.
func ParseFloat(lit string) (float64, error) {
	s := stripUnderscore(lit)
	if s == "" {
		return 0, fmt.Errorf("empty float")
	}
	return strconv.ParseFloat(s, 64)
}

func stripUnderscore(s string) string {
	if strings.IndexByte(s, '_') < 0 {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			b = append(b, s[i])
		}
	}
	return string(b)
}
