version 1.2

task hello_task {
    input {
        File infile
        String pattern
    }
    
    command <<<
        grep -E "${pattern}" "${infile}" || echo "no matches"
    >>>
    
    requirements {
        container: "ubuntu:latest"
    }
    
    output {
        String result = read_string(stdout())
    }
}

workflow hello {
    input {
        File infile
        String pattern
    }
    
    call hello_task { input: infile = infile, pattern = pattern }
    
    output {
        String result = hello_task.result
    }
}