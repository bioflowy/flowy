version 1.2

task read_object {
  command <<<
    python <<CODE
    print('\t'.join(["key_{}".format(i) for i in range(3)]))
    print('\t'.join(["value_{}".format(i) for i in range(3)]))
    CODE
  >>>

  output {
    Object my_obj = read_object(stdout())
  }

  requirements {
    container: "python:latest"
  }
}