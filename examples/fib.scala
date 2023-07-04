// expect: 55

def fib(n: Int): Int =
  if n < 2 then n else fib(n - 1) + fib(n - 2)

def main(): Int = fib(10)
