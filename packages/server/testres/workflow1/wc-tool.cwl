#!/usr/bin/env cwl-runner
class: CommandLineTool
cwlVersion: v1.2
doc: "count words."
inputs:
  file1: File

outputs:
  output:
    type: File
    outputBinding: { glob: output }

baseCommand: [sed, -n, $=]

stdin: $(inputs.file1.path)
stdout: output
