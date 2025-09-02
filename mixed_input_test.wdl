version 1.2

task mixed_input_test {
  input {
    String provided_input
    Array[String] default_array = ["default1", "default2"]
    Int default_number = 42
  }

  command <<<
    echo "Provided: ~{provided_input}"
    echo "Array: ~{sep(',', default_array)}"
    echo "Number: ~{default_number}"
  >>>

  output {
    String result = read_string(stdout())
  }
  
  requirements {
    container: "ubuntu:latest"
  }
}