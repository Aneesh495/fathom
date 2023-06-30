// expect: 3
// expect-out: 13

class Box[T](val value: T) {
  def map[U](f: T => U): Box[U] = Box(f(value))
}

def main(): Int = {
  val b = Box(3)
  println(b.map((x: Int) => x + 10).value)
  b.value
}
