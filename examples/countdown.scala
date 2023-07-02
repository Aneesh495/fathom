// expect: 0

def down(n: Int): Int = {
  var i = n
  while i > 0 do i = i - 1
  i
}

def main(): Int = down(4)
