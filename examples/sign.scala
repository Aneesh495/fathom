// expect: -1

def sign(n: Int): Int =
  if n < 0 then -1 else if n > 0 then 1 else 0

def main(): Int = sign(0 - 5)
