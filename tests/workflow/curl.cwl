cwlVersion: v1.0
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