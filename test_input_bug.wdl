version 1.2

task simple_task {
    input {
        String message
    }
    
    command {
        echo "${message}"
    }
    
    output {
        String result = read_string(stdout())
    }
}

workflow test_input_workflow {
    input {
        String message
    }
    
    call simple_task { input: message = message }
    
    output {
        String result = simple_task.result
    }
}