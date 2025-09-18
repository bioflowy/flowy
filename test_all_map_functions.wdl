version 1.2

workflow TestAllMapFunctions {
    # Test data
    Map[String, Int] simple_map = {"a": 1, "b": 2, "c": 3}
    Array[Pair[String, Int]] test_pairs = [("x", 10), ("y", 20), ("x", 30)]
    Array[Pair[String, Int]] unique_pairs = [("p", 100), ("q", 200)]

    # Test keys function
    Array[String] map_keys = keys(simple_map)

    # Test values function
    Array[Int] map_values = values(simple_map)

    # Test contains_key function
    Boolean has_a = contains_key(simple_map, "a")
    Boolean has_d = contains_key(simple_map, "d")

    # Test as_pairs function
    Array[Pair[String, Int]] pairs_from_map = as_pairs(simple_map)

    # Test as_map function (should work with unique keys)
    Map[String, Int] map_from_pairs = as_map(unique_pairs)

    # Test collect_by_key function (handles duplicate keys)
    Map[String, Array[Int]] collected = collect_by_key(test_pairs)

    output {
        Array[String] out_keys = map_keys
        Array[Int] out_values = map_values
        Boolean out_has_a = has_a
        Boolean out_has_d = has_d
        Array[Pair[String, Int]] out_pairs = pairs_from_map
        Map[String, Int] out_map = map_from_pairs
        Map[String, Array[Int]] out_collected = collected
    }
}