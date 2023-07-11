// expect: 9

trait Ord[A] {
  def compare(x: A, y: A): Int
}

given intOrd: Ord[Int] with {
  def compare(x: Int, y: Int): Int =
    if x < y then -1 else if x > y then 1 else 0
}

def max(x: Int, y: Int)(using ord: Ord[Int]): Int =
  if ord.compare(x, y) < 0 then y else x

def main(): Int = max(3, 9)
