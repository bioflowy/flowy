version 1.2

workflow scatter_if_wf {
  input {
    Array[String] items
  }

  scatter (item in items) {
    String scatter_out = item
  }

  Boolean flag = true

  if (flag) {
    String if_out = "hello"
  }
}
