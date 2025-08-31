version 1.2

task simple {
    command {
        echo "hello"
    }
    
    output {
        String result = read_string(stdout())
    }
}

workflow test_workflow {
    call simple
    
    output {
        String result = simple.result
    }
}