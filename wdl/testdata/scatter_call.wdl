version 1.2

task upper {
  input { String s }
  command { echo ~{s} }
  output { String result = read_string(stdout()) }
}

workflow scatterCallWf {
  input { Array[String] items }
  scatter (item in items) {
    call upper { input: s = item }
  }
  output { Array[String] results = upper.result }
}
