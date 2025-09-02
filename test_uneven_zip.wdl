version 1.2

workflow test_uneven_zip {
  Array[String] short_array = ["A", "B"]
  Array[Int] long_array = [1, 2, 3, 4, 5]
  
  output {
    Array[Pair[String, Int]] zipped = zip(short_array, long_array)
  }
}