version 1.2

task simple {
    command <<<
        echo "hello world"
    >>>
    
    output {
        String result = read_string(stdout())
    }
    
    requirements {
        container: "ubuntu:20.04"
        cpu: 1
        memory: "1 GB"
    }
}

workflow test_workflow {
    call simple
    
    output {
        String result = simple.result
    }
}