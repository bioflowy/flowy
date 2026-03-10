version 1.2

task sayHello {
  input {
    String name
  }
  command {
    echo "Hello ~{name}"
  }
  output {
    String greeting = read_string(stdout())
  }
}

workflow helloWorld {
  input {
    String who = "World"
  }
  call sayHello { input: name = who }
  output {
    String result = sayHello.greeting
  }
}
