// expect: blue

enum Color {
  case Red
  case Blue
}

def name(c: Color): String = c match {
  case Red => "red"
  case Blue => "blue"
}

def main(): String = name(Blue)
