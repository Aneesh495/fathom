// expect: true
// expect-out: left

def side(flag: Boolean, label: String): Boolean = {
  println(label)
  flag
}

def main(): Boolean = side(true, "left") || side(false, "right")
