version 1.2

# Example demonstrating WDL 1.2 requirements section
task cpu_intensive {
    input {
        String input_data = "sample data"
        Int cpu_count = 4
        String memory_size = "8 GiB"
    }
    
    command {
        echo "Processing ${input_data} with ${cpu_count} CPUs"
        # Simulate CPU-intensive work
        echo "Task completed"
    }
    
    output {
        String result = read_string(stdout())
    }
    
    requirements {
        container: "ubuntu:20.04"
        cpu: cpu_count
        memory: memory_size
        maxRetries: 3
        returnCodes: [0, 1]
    }
}

task with_gpu {
    input {
        String model_path
    }
    
    command {
        echo "Training model from ${model_path}"
    }
    
    output {
        String training_log = read_string(stdout())
    }
    
    requirements {
        container: "tensorflow/tensorflow:2.8.0-gpu"
        cpu: 2
        memory: "16 GB"
        gpu: true
        gpuType: "nvidia-tesla-v100"
        gpuCount: 1
        disks: "100 GB"
    }
    
    hints {
        preemptible: 1
        maxCpu: 8
        localization_optional: false
    }
}

workflow requirements_demo {
    input {
        String data = "test input"
        String model = "/path/to/model"
    }
    
    call cpu_intensive { input: input_data = data }
    call with_gpu { input: model_path = model }
    
    output {
        String cpu_result = cpu_intensive.result
        String gpu_result = with_gpu.training_log
    }
}