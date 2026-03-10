version 1.2

task forward_ref_task {
  String greeting = "Hello " + name
  String name = "World"

  command {}

  output {
    String result = greeting
  }
}
