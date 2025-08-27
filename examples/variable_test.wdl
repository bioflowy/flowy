version 1.0

task test_variables {
    input {
        String name = "World"
        String greeting = "Hello"
    }
    
    command {
        echo "${greeting}, ${name}!"
        echo "This is a test with ${name}"
    }
    
    output {
        File result = stdout()
    }
}

workflow test_workflow {
    input {
        String person = "Test User"
        String message = "Greetings"
    }
    
    call test_variables {
        input: name = person, greeting = message
    }
    
    output {
        File output_file = test_variables.result
    }
}