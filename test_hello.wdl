version 1.0

task hello {
    command {
        echo "Hello, World!"
    }
    
    output {
        File stdout_file = stdout()
        String message = read_string(stdout())
    }
}

workflow main {
    call hello
    
    output {
        String result = hello.message
    }
}