version 1.2

task greet {
  input { String name }
  command { echo "Hi ~{name}" }
  output { String message = read_string(stdout()) }
}

workflow callOutputWf {
  call greet { input: name = "Alice" }
  output { String msg = greet.message }
}
