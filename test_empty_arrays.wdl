version 1.2

workflow test_empty_arrays {
  Array[String] empty_strings = []
  Array[Int] empty_ints = []
  Array[Array[String]] empty_matrix = []
  
  output {
    Array[Pair[String, Int]] empty_zip = zip(empty_strings, empty_ints)
    Array[Pair[String, Int]] empty_cross = cross(empty_strings, empty_ints)
    Array[Array[String]] empty_transpose = transpose(empty_matrix)
    Pair[Array[String], Array[Int]] empty_unzip = unzip([])
  }
}