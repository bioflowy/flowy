version 1.2

task type_error {
  input {
    Int x
  }
  command <<<
    echo ~{x}
  >>>
  output {
    String result = x
  }
}
