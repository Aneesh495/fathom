// expect: 4

extension (x: Int)
  def abs: Int = if x < 0 then -x else x

def main(): Int = (-4).abs
