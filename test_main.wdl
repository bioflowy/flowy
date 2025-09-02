version 1.0

import "test_imports.wdl" as lib

workflow test_workflow {
    call lib.hello { input: name = "Rust" }
    output {
        String result = hello.greeting
    }
}