version 1.2

struct Address {
  String street
  String city
}

struct Person {
  String name
  Address addr
  Int age
}

task processData {
  input {
    Array[String]+ names
    Map[String, Int] scores
    Pair[Boolean, File] flagged
  }
  command {
    echo "processing"
  }
}
