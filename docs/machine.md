# Machine

The bytecode is a stack machine with named locals. Instructions and their immediate widths live in one table. The assembler, the decoder, the verifier, and the disassembler all read that table, so a new opcode cannot be encoded one way and decoded another.

## Calls

A direct call pops its arguments into the callee's locals, in order. A virtual call does the same, then looks up the method name along the receiver's linearization and enters the first body it finds. Extension methods are direct calls: the receiver is an extra leading argument, which may be an unboxed integer.

A closure records a function index and the captured values. `callclos` pops the arguments and the closure, then enters the function with the arguments in the parameter slots and the captures in the slots after them.

`return` yields the top of the callee's stack to the caller. `main` is an ordinary function. Its result is the program result.

## Objects

`new` allocates a reference with the class's field count. Fields are laid out parent-first, then the class's own `val` and `var` members, excluding `this`. A constructor is `new`, then a `putfield` for each argument. Case objects and given instances are globals, allocated once when the machine starts.

Pattern matching compiles to `classeq` and field loads. A failure jumps to the next case. If every case fails, `trap` raises a match error.

## Collector

References are indexes into a heap slice. Index 0 is empty. The collector clears mark bits, marks every global and every slot of every live frame, and adds unmarked indexes to a free list. The next allocation reuses a free index before it grows the slice.

Collection runs before an allocation once the allocation counter passes a threshold. The object about to be allocated is not a root yet, so the collection happens first and the new reference is pushed afterward. A test builds a rooted cell, allocates garbage, collects, and checks that only the root and its child remain.

## Agreement

`examples/` is executed by the walker and by the machine. The test compares both the returned value and the text written by `println`. A jump that does not land on an instruction boundary is rejected before execution. `fathom dis` prints the functions if you want to see the encoding.
