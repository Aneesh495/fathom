// expect: 15

def sumTo(n: Int): Int = {
  var i = 1
  var acc = 0
  while i <= n do {
    acc = acc + i
    i = i + 1
  }
  acc
}

def main(): Int = sumTo(5)
