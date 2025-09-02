version 1.2

task output_reference_test {
  input {
    Int num = 2
  }

  command <<<
  echo "file1" > file1.txt
  echo "file2" > file2.txt
  >>>

  output {
    Array[File] files = glob("*.txt")
    Int file_count = length(files)
  }
}