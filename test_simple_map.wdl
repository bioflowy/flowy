version 1.2

workflow TestSimpleMap {
    Map[String, Int] simple_map = {"a": 1, "b": 2, "c": 3}

    # Test contains_key function
    Boolean has_a = contains_key(simple_map, "a")
    Boolean has_d = contains_key(simple_map, "d")

    # Test values function
    Array[Int] map_values = values(simple_map)

    output {
        Boolean out_has_a = has_a
        Boolean out_has_d = has_d
        Array[Int] out_values = map_values
    }
}