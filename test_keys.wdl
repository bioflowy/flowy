version 1.2

struct Name {
  String first
  String last
}

workflow test_keys {
  input {
    Map[String, Int] x = {"a": 1, "b": 2, "c": 3}
    Map[String, Pair[File, File]] str_to_files = {
      "a": ("a.bam", "a.bai"), 
      "b": ("b.bam", "b.bai")
    }
    Name name = Name {
      first: "John",
      last: "Doe"
    }
  }

  scatter (item in as_pairs(str_to_files)) {
    String key = item.left
  }

  Array[String] str_to_files_keys = key
  Array[String] expected = ["a", "b", "c"]
  Array[String] expectedKeys = ["first", "last"]

  output {
    Boolean is_true1 = length(keys(x)) == 3 && keys(x) == expected
    Boolean is_true2 = str_to_files_keys == keys(str_to_files)
    Boolean is_true3 = length(keys(name)) == 2 && keys(name) == expectedKeys
  }
}