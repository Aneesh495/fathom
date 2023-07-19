// expect: 32

def power(b: Int, e: Int): Int =
  if e == 0 then 1 else b * power(b, e - 1)

def main(): Int = power(2, 5)
