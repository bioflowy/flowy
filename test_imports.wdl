version 1.0

task hello {
    input {
        String name = "World"
    }
    
    command {
        echo "Hello, ~{name}!"
    }
    
    output {
        String greeting = stdout()
    }
}