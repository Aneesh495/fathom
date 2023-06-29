// expect: 13

def adder(n: Int): Int => Int = (x: Int) => x + n

def main(): Int = adder(10)(3)
