version 1.2

struct Person {
  String name
  Map[String, File] assay_data
}

task test_struct_access {
  input {
    Person person
  }
  
  String name_value = person.name
  Map[String, File] assay_value = person.assay_data
  
  command <<<
    echo "Name: ~{name_value}"
  >>>
  
  output {
    String message = read_string(stdout())
  }
}