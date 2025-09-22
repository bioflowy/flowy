version 1.2

task gen_files {
  input {
    Int num_files
  }

  command <<<
    for i in {1..~{num_files}}; do
      printf ${i} > a_file_${i}.txt
    done
    mkdir a_dir
    touch a_dir/a_inner.txt
  >>>

  output {  
    Array[File] files = glob("a_*")
    Int glob_len = length(files)
  }
}