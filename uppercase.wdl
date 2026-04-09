version 1.2

task uppercase_task {
  input {
    File infile
  }

  command <<<
    tr '[:lower:]' '[:upper:]' < ~{infile} > output.txt
  >>>

  output {
    File outfile = "output.txt"
  }

  runtime {
    docker: "ubuntu:24.04"
  }
}

workflow uppercase {
  input {
    File infile
  }

  call uppercase_task {
    input:
      infile = infile
  }

  output {
    File result = uppercase_task.outfile
  }
}
