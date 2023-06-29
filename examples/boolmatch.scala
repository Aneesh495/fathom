// expect: yes

def word(b: Boolean): String = b match {
  case true => "yes"
  case false => "no"
}

def main(): String = word(true)
