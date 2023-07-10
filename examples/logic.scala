// expect: false
// expect-out: left

def side(flag: Boolean, label: String): Boolean = {
  println(label)
  flag
}

def main(): Boolean = side(false, "left") && side(true, "right")
