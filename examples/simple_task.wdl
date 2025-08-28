version 1.0

task process_file {
    input {
        String input_text = "Hello World"
        String output_filename = "output.txt"
    }

    command <<<
        echo "Processing: ${input_text}"
        echo "${input_text}" | wc -w > word_count.txt
        echo "Input: ${input_text}" > ${output_filename}
        echo "Word count: $(cat word_count.txt)" >> ${output_filename}
    >>>

    output {
        File result = output_filename
        String summary = stdout()
        Int word_count = read_int("word_count.txt")
    }
}