version 1.2

task simple {
    input {
        String msg = "hello"
    }
    
    command {
        echo "${msg}"
    }
    
    output {
        String result = stdout()
    }
    
    runtime {
        docker: "ubuntu:20.04"
    }
}