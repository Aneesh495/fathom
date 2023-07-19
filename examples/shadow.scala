// expect: 3

def main(): Int = {
  val x = 1
  val y = {
    val x = 3
    x
  }
  y
}
