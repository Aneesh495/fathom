// expect: 6

enum Option[+A] {
  case None
  case Some(value: A)
  def map[B](f: A => B): Option[B] = this match {
    case None => None
    case Some(v) => Some(f(v))
  }
}

def main(): Int = Some(5).map((x: Int) => x + 1) match {
  case Some(v) => v
  case None => 0
}
