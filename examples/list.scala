// expect: 11

enum List[+A] {
  case Nil
  case Cons(head: A, tail: List[A])
  def map[B](f: A => B): List[B] = this match {
    case Nil => Nil
    case Cons(h, t) => Cons(f(h), t.map(f))
  }
}

def main(): Int = Cons(1, Cons(2, Nil)).map((x: Int) => x + 10) match {
  case Cons(h, _) => h
  case Nil => 0
}
