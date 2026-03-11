version 1.2

task basic_report {
  input {
    String message
  }
  command <<<
    echo "Basic report: ~{message}"
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String report = read_string(stdout())
  }
}

task extended_report {
  input {
    String message
  }
  command <<<
    echo "Extended report: ~{message} (with additional analysis)"
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String report = read_string(stdout())
  }
}

workflow conditional_example {
  input {
    Boolean run_extended
    String message
  }
  call basic_report { input: message = message }
  if (run_extended) {
    call extended_report { input: message = message }
  }
}
