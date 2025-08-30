version 1.0

task test_read_string {
    input {
        String filename
    }
    
    command <<<
        echo "Hello from stdout!"
        echo "Hello from stderr!" >&2
    >>>
    
    output {
        String stdout_content = read_string(stdout())
        String stderr_content = read_string(stderr())
    }
}

workflow main {
    call test_read_string { input: filename = "test_read_string.txt" }
    
    output {
        String stdout_result = test_read_string.stdout_content  
        String stderr_result = test_read_string.stderr_content
    }
}