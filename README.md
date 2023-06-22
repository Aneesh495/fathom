# Fathom

A Scala 3 kernel, written in Go.

Fathom is a compiler and abstract machine for a studied subset of Scala 3. The subset is built around the parts of the language that make its type system what it is: path-dependent types, intersection and union types, given and using clauses, sealed hierarchies, and Scala linearization. Surface programs are elaborated into interned types, checked by a fuel-bounded coinductive subtype relation, and executed two ways. A reference walker defines the semantics. A stack machine with a precise mark-sweep collector has to agree with it.

```
source
  |  scanner, newline rules, interpolators
  v
AST
  |  namer, signatures, givens, patterns
  v
symbols and interned types
  |  reference walker          bytecode compiler
  v                            v
values                       stack machine
                               |  mark-sweep
                               v
                             result
```

## Start

Go 1.25 or newer is enough. There are no other dependencies.

```bash
go test ./...
go run ./cmd/fathom run examples/fact.scala
go run ./cmd/fathom run examples/list.scala
go run ./cmd/fathom dis examples/fact.scala
```

`fathom check` stops after elaboration. `fathom eval` uses the reference walker. `fathom run` compiles to bytecode and executes that.

## What the kernel contains

* Classes, traits, objects, and enums, with Scala linearization.
* Type parameters with variance, bounds, and local instantiation at calls.
* Path-dependent types and type members, opened to their bounds under a fuel budget.
* Union and intersection types.
* Given and using resolution, with specificity and a divergence check.
* Extension methods.
* Sealed exhaustiveness for enums and for `Boolean`.
* Closures, including captured locals.
* A reference evaluator and a bytecode machine that are required to agree.

`Int` is a 64-bit signed integer. `Double` is IEEE-754 binary64. Strings are Unicode. The subset is the brace dialect: a newline ends a statement except before an operator, `.`, `(`, or `[`.

The language document in `docs/language.md` lists the grammar. `docs/typing.md` describes the subtype relation. `docs/machine.md` describes the bytecode and the collector.

## Layout

```
cmd/fathom            command line
internal/scanner      lexer and semicolon insertion
internal/parser       recursive descent
internal/ast          surface syntax
internal/types        symbols, interning, subtyping, linearization
internal/check        elaboration
internal/eval         reference walker
internal/compile      bytecode lowering
internal/bc           instruction encoding
internal/vm           machine and collector
internal/driver       pipeline used by the command and the example tests
examples              programs both engines run
docs                  language, typing, and machine notes
```

## An example

```scala
enum List[+A] {
  case Nil
  case Cons(head: A, tail: List[A])
  def map[B](f: A => B): List[B] = this match {
    case Nil => Nil
    case Cons(h, t) => Cons(f(h), t.map(f))
  }
}
```

`Cons` is its own class. Its type parameter is a different skolem from `List`'s, and matching a `List[T]` substitutes one for the other. `map` on a `Cons[Int]` is still `List`'s method: the receiver's parent clause supplies `Int` for `A`. The bytecode for that call is virtual. The collector traces the spine if it is still reachable, and reclaims it when it is not.
