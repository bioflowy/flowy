version 1.2

task say_hello {
  input {
    String name
  }
  command <<<
    echo "Hello, ~{name}!"
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String greeting = read_string(stdout())
  }
}

workflow greet {
  input {
    String name = "world"
  }
  call say_hello { input: name = name }
}
