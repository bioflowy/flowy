version 1.2

task process_sample {
  input {
    String sample_name
  }
  command <<<
    echo "Processing: ~{sample_name}"
    echo "~{sample_name}_result"
  >>>
  runtime {
    docker: "ubuntu:22.04"
  }
  output {
    String result = read_string(stdout())
  }
}

workflow scatter_example {
  input {
    Array[String] samples
  }
  scatter (sample in samples) {
    call process_sample { input: sample_name = sample }
  }
  output {
    Array[String] results = process_sample.result
  }
}
