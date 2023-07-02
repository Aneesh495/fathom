// expect: 6

def applyTwice(f: Int => Int, n: Int): Int = f(f(n))

def main(): Int = applyTwice((x: Int) => x + 1, 4)
