# Typing

The subtype checker is a fuel-bounded coinductive relation. A yes answer is one the rules derived. Running out of fuel is a no. That keeps the checker sound in the presence of recursive bounds: it may reject a program it does not have the budget to accept, and it does not accept a program the rules reject.

## Ground rules

* Everything is a subtype of `Any`, and `Nothing` is a subtype of everything.
* `Null` is a subtype of reference types and not of value classes.
* A class is a subtype of each of its parents, after substituting its type arguments.
* Variance decides how type arguments compare: covariant, contravariant, or invariant.
* Functions compare contravariantly on the left and covariantly on the right.
* `A <: A | B`, and `A | B <: T` only when both arms are.
* `A & B <: A`, and `T <: A & B` only when both sides are.
* A literal singleton such as `1` is a subtype of its widened class, and of itself.

These rules are tested as a finite lattice, including transitivity over the set of ground types the universe defines.

## Paths

A stable path is a parameter, a `val`, `this`, or a chain of stable fields. A selection `p.A` opens to the upper bound of `A` when it is on the left of `<:`, and to the lower bound when it is on the right. The opened bound is substituted so that `this` becomes `p`. An assumption stack cuts a bound that mentions itself. Fuel cuts a chain that does not contract.

An abstract type member therefore satisfies `p.A <: Any` and does not satisfy `p.A <: Int` unless its upper bound does. Two different prefixes do not share an abstract member: `a.Elem` and `b.Elem` are different types when `a` and `b` are different paths.

## Inference

A call with omitted type arguments creates a fresh variable for each parameter. Arguments are constrained against the substituted parameter types. A variable on the right of `<:` accumulates a lower bound. A variable on the left accumulates an upper bound. At the end, a variable becomes its lower bound if that bound is not `Nothing`, otherwise its upper bound if that bound is not `Any`. A variable with no bound becomes an error, so an unconstrained type parameter cannot leak into a typed program.

`id(1)` for `def id[A](x: A): A` therefore has type `Int`.

## Inheritance and cases

`Cons` in `enum List[+A]` has its own skolem for `A`, and it extends `List` at that skolem. Matching `Cons` against a scrutinee of type `List[T]` substitutes the case skolem with `T`, so the bound fields have type `T` rather than a different `A` that only prints the same way.

A method found on a parent is instantiated from the parent clause of the receiver. `Cons[Int].map` sees `List`'s `A` as `Int`, because `Cons[Int]` extends `List[Int]`.

## Givens

Search walks outward through lexical scopes, then the companions of the type constructors in the target. A candidate's type parameters are fresh variables, constrained so that the candidate's result matches the target. Using parameters of the candidate are resolved recursively. The set of targets already being searched rejects a given that requires itself.

If several candidates succeed, the more specific one wins: its type is a subtype of the other's, and not the other way around. Two incomparable successes are an ambiguity error. The chosen symbol is stored on the call as evidence. Both executors read that evidence, so they cannot pick different instances.

## Linearization

For `C extends C1 with C2 with Cn`, the linearization is `C` followed by the merge of `L(Cn) ... L(C1)`. Merge repeatedly takes the first head that does not occur in the tail of any list. That is the C3 condition Scala uses. `class D extends B with C`, when `B` and `C` both extend `A`, linearizes as `D, C, B, A, AnyRef, Any`.

A parent cycle is cut rather than followed forever. The namer does not try to give a cycle a meaning beyond that.
