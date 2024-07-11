cwlVersion: v1.0
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