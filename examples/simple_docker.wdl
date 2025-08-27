version 1.0

task simple_hello {
    input {
        String name = "World"
    }
    
    command {
        echo "Hello, ${name}!"
    }
    
    runtime {
        docker: "ubuntu:20.04"
    }
    
    output {
        File greeting = stdout()
    }
}

workflow simple_workflow {
    call simple_hello
    
    output {
        File result = simple_hello.greeting
    }
}