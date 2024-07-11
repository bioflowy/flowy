cwlVersion: v1.0
class: Workflow

inputs:
  directory:
    type: string
    default: "."
  sort_column:
    type: string
    default: "9"  # デフォルトでファイル名でソート

outputs:
  sorted_output:
    type: File
    outputSource: sort_step/sorted_file

steps:
  ls_step:
    run:
      class: CommandLineTool
      baseCommand: [ls, -l]
      inputs:
        dir:
          type: string
          inputBinding:
            position: 1
      outputs:
        ls_output:
          type: stdout
          streamable: true
    in:
      dir: directory
    out: [ls_output]

  sort_step:
    run:
      class: CommandLineTool
      baseCommand: [sort]
      inputs:
        sort_input:
          type: File
          streamable: true
          inputBinding:
            position: 1
        key:
          type: string
          inputBinding:
            prefix: -k
            position: 2
      outputs:
        sorted_file:
          type: File
          outputBinding:
            glob: sorted_output.txt
      stdout: sorted_output.txt
    in:
      sort_input: ls_step/ls_output
      key: sort_column
    out: [sorted_file]