version 1.0

task hello {
    input {
        String name = "World"
    }

    command {
        echo "Hello, ${name}!"
    }

    output {
        String message = stdout()
    }

    runtime {
        docker: "ubuntu:20.04"
    }
}

workflow hello_world {
    input {
        String greeting_name = "WDL"
    }

    call hello { input: name = greeting_name }

    output {
        String greeting = hello.message
    }
}