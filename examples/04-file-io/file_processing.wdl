version 1.2

task write_data {
  input {
    String content
  }
  command <<<
    echo "~{content}" > output.txt
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    File result = "output.txt"
  }
}

task read_data {
  input {
    File input_file
  }
  command <<<
    wc -l < ~{input_file}
    cat ~{input_file}
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String data = read_string(stdout())
  }
}

workflow file_processing {
  input {
    String content
  }
  call write_data { input: content = content }
  call read_data { input: input_file = write_data.result }
}
