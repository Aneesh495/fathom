// expect: 3

trait Num {
  def n: Int
}

class Three extends Num {
  def n: Int = 3
}

def main(): Int = (new Three()).n
