version 1.2

workflow test_length {
  Array[Int] xs = [1, 2, 3]
  Array[String] ys = ["a", "b", "c"]
  Array[String] zs = []
  Map[String, Int] m = {"a": 1, "b": 2}
  String s = "ABCDE"

  output {
    Int xlen = length(xs)
    Int ylen = length(ys)
    Int zlen = length(zs)
    Int mlen = length(m)
    Int slen = length(s)
  }
}