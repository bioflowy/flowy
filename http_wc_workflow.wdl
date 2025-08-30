version 1.0

task download_and_count {
    input {
        String url
        String output_name = "downloaded_file.txt"
    }

    command {
        set -e
        echo "Downloading from ${url}..."
        curl -L -o ${output_name} "${url}"
        echo "Running word count..."
        wc ${output_name} > wc_result.txt
        cat wc_result.txt
    }

    output {
        File downloaded_file = output_name
        File wc_result_file = "wc_result.txt"
        String wc_result = read_string("wc_result.txt")
    }
}

workflow http_wc_workflow {
    input {
        String file_url
    }

    call download_and_count {
        input:
            url = file_url
    }

    output {
        File downloaded_file = download_and_count.downloaded_file
        String word_count = download_and_count.wc_result
    }
}