# Language

Fathom parses a brace dialect of a Scala 3 subset. The examples in `examples/` are the executable specification of the surface syntax. This note is the map.

## Programs

A file is an optional package, imports, and a sequence of declarations. `def main` with no arguments, or a method marked `@main`, is the entry point. `main` may return any type. `println` writes a value and a newline.

## Declarations

```
class Name[TParams](params) extends Parent with Parent { stats }
trait Name[TParams] extends Parent { stats }
object Name extends Parent { stats }
enum Name[TParams] { case C; case D(x: T); stats }
def name[TParams](params)(using params): Type = expr
val name: Type = expr
var name: Type = expr
type Name = Type
type Name >: Lo <: Hi
given name: Type with { stats }
given name: Type = expr
extension (x: Type) { stats }
```

Case class parameters and enum case parameters are immutable fields. A parameter written `var` is mutable. Unadorned class parameters of a normal class are constructor arguments only.

Enums elaborate to a sealed trait, a companion object, a case object for each parameterless case, and a case class for each parameterized case. A parameterless case of a covariant enum has the bottom type in that parameter, so `Nil` inhabits `List[A]` for every `A`.

## Expressions

Literals, names, selection, application, infix and prefix operators, tuples, blocks, `if`/`then`/`else`, `while`/`do`, `do`/`while`, `match`, `new`, lambdas, ascription, return, throw, and `s` and `raw` interpolation.

Infix operators follow Scala's first-character precedence. An operator that ends in `:` is right-associative and is invoked on its right operand, so `1 :: xs` is `xs.::(1)`.

`&&` and `||` are short-circuit. They are not method calls.

A lambda needs parameter types, or an expected function type. `x => x` is a lambda. `(x: Int) => x + 1` is a lambda. A bare `x: Int => Int` is an ascription whose type is a function type.

`for` comprehensions desugar to `map`, `flatMap`, `foreach`, and `withFilter`. A generator may bind a name. A guard is `if expr`.

## Types

```
A
A[T, U]
A => B
(A, B)
A | B
A & B
path.Type
path.type
T#A
=> T
```

`|` and `&` are union and intersection. Function types are contravariant in the argument and covariant in the result. A tuple type is `Tuple2`, `Tuple3`, or `Tuple4`.

## Patterns

A lowercase name is a binder. An uppercase name or a dotted path is a constructor or a stable name. Literals, tuples, typed patterns, alternatives, and `name @ pattern` are accepted. A `match` on a sealed type or on `Boolean` must cover every case, unless a wildcard or a variable is present. Guards are not considered covering.

## Numeric tower

`Int` is signed 64-bit. `Double` is IEEE-754 binary64. Integer division truncates toward zero. Division by zero is a runtime error and carries a stack.

## What is intentionally small

Higher-kinded type constructors are parsed as type parameters with parameters (`F[_]`), and the checker records them. It does not infer kinds beyond that. By-name parameters are accepted syntactically and treated as the underlying type. Indentation-sensitive template bodies are supported after `:`. XML literals, macros, and inline are outside the kernel.
