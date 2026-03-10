version 1.2

task type_mismatch_task {
  input {
    Int count
  }

  String bad_assign = count

  command {}
}
