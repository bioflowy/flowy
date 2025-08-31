version 1.2

task simple_task {
    input {
        String msg = "hello"
    }
    
    command <<<
        echo ${msg}
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

workflow simple_workflow {
    call simple_task
    
    output {
        String result = simple_task.result
    }
}