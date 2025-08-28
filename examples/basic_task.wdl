version 1.0

task echo_task {
    input {
        String message = "Hello World"
    }

    command {
        echo "${message}"
    }

    output {
        String result = message
    }
}