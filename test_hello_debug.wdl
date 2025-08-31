version 1.2

task hello_task {
    input {
        File infile
        String pattern
    }
    
    command <<<
        echo "Input file: ${infile}"
        echo "Pattern: ${pattern}"
        cat "${infile}"
        echo "Running grep command:"
        grep -E "${pattern}" "${infile}" || echo "No matches found"
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