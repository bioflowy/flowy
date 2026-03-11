version 1.2

task step1_parse {
  input {
    String raw_data
  }
  command <<<
    echo "~{raw_data}"
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String parsed = read_string(stdout())
  }
}

task step2_transform {
  input {
    String data
  }
  command <<<
    echo "~{data}" | tr '[:lower:]' '[:upper:]'
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String transformed = read_string(stdout())
  }
}

task step3_format {
  input {
    String data
  }
  command <<<
    echo "=== ~{data} ==="
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String formatted = read_string(stdout())
  }
}

workflow pipeline {
  input {
    String raw_data
  }
  call step1_parse { input: raw_data = raw_data }
  call step2_transform { input: data = step1_parse.parsed }
  call step3_format { input: data = step2_transform.transformed }
}
