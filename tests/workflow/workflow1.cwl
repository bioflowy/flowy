cwlVersion: v1.0
class: Workflow
inputs:
  url: string

steps:
  curl_step:
    run:
      class: CommandLineTool
      baseCommand: ['curl','-o','output.html']
      inputs:
        url:
          type: string
          inputBinding:
            position: 1
      outputs:
        html_file:
          type: File
          streamable: true
          outputBinding:
            glob: output.html
    in:
      url: url
    out: [html_file]

  wc_step:
    run: 
      class: CommandLineTool
      baseCommand: wc
      inputs:
        input_file:
          type: File
          streamable: true
          inputBinding:
            position: 1
      outputs:
        wc_output:
          type: stdout
      stdout: wc_result.txt
    in:
      input_file: curl_step/html_file
    out: [wc_output]

outputs:
  wc_result:
    type: File
    outputSource: wc_step/wc_output