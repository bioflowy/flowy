version 1.2

task hello {
  input {
    String name
  }
  command <<<
    echo "Hello, ~{name}!"
  >>>
  output {
    String greeting = read_string(stdout())
  }
}
