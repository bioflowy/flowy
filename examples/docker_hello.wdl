version 1.0

task hello_docker {
    input {
        String name = "World"
    }
    
    command <<<
        echo "Hello, ${name} from Docker!"
        echo "Container hostname: $(hostname)"
        echo "Current time: $(date)"
    >>>
    
    runtime {
        docker: "ubuntu:20.04"
        memory: "512 MB"
        cpu: 1
    }
    
    output {
        File greeting = stdout()
    }
}

workflow docker_hello_workflow {
    input {
        String person_name = "Docker User"
    }
    
    call hello_docker {
        input: name = person_name
    }
    
    output {
        File result = hello_docker.greeting
    }
}