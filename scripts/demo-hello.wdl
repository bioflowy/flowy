version 1.2

# Simple WDL task used by the v1.3 end-to-end demo script.
# Writes a greeting to a file and returns it as a String output.
task hello {
  input {
    String name = "World"
  }
  command <<<
    echo "Hello, ~{name}!" > greeting.txt
  >>>
  output {
    String greeting = read_string("greeting.txt")
  }
  runtime {
    docker: "ubuntu:22.04"
  }
}
