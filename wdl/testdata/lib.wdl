version 1.2

task greet {
  input {
    String person
  }
  command {
    echo "Hi ~{person}"
  }
  output {
    String message = read_string(stdout())
  }
}
