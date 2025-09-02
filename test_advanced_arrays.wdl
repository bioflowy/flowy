version 1.2

workflow test_advanced_arrays {
  Array[String] names = ["Alice", "Bob", "Charlie"]
  Array[Int] ages = [25, 30, 35]
  Array[Array[Int]] matrix = [[1, 2, 3], [4, 5, 6]]
  
  output {
    Array[Pair[String, Int]] zipped = zip(names, ages)
    Array[Pair[String, Int]] crossed = cross(["A", "B"], [1, 2])
    Array[Array[Int]] transposed = transpose(matrix)
    Pair[Array[String], Array[Int]] unzipped = unzip(zip(names, ages))
  }
}