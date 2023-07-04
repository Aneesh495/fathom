// expect: 6

def gcd(a: Int, b: Int): Int =
  if b == 0 then a else gcd(b, a % b)

def main(): Int = gcd(54, 24)
