version 1.2

struct MyType {
  String s
}

workflow test_struct {
  MyType my = MyType { s: "hello" }
  
  output {
    MyType result = my
  }
}