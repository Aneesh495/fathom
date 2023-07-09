// expect: 3

enum List[+A] {
  case Nil
  case Cons(head: A, tail: List[A])
  def length: Int = this match {
    case Nil => 0
    case Cons(_, t) => 1 + t.length
  }
}

def main(): Int = Cons(1, Cons(2, Cons(3, Nil))).length
