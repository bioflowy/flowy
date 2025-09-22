version 1.2

task read_objects {
  command <<<
    python <<CODE
    print('\t'.join(["key_{}".format(i) for i in range(3)]))
    print('\t'.join(["value_A{}".format(i) for i in range(3)]))
    print('\t'.join(["value_B{}".format(i) for i in range(3)]))
    print('\t'.join(["value_C{}".format(i) for i in range(3)]))
    CODE
  >>>

  output {
    Array[Object] my_obj = read_objects(stdout())
  }

  requirements {
    container: "python:latest"
  }
}