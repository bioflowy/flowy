version 1.2

struct Test {
    String name
}

workflow debug {
    Test t = {"name": "hello"}
    output {
        Test result = t
    }
}