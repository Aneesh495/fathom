// expect: 4

def second(p: Tuple2[Int, Int]): Int = p._2

def main(): Int = second((1, 4))
