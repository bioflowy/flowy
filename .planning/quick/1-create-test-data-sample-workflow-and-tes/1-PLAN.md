---
phase: quick
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - examples/README.md
  - examples/01-hello-world/hello.wdl
  - examples/01-hello-world/inputs.json
  - examples/02-scatter-gather/scatter.wdl
  - examples/02-scatter-gather/inputs.json
  - examples/03-conditional/conditional.wdl
  - examples/03-conditional/inputs.json
  - examples/04-file-io/file_processing.wdl
  - examples/04-file-io/inputs.json
  - examples/05-multi-step/pipeline.wdl
  - examples/05-multi-step/inputs.json
autonomous: true
requirements: []
must_haves:
  truths:
    - "User can build flowy and run each example workflow end-to-end"
    - "Each example demonstrates a distinct flowy feature"
    - "README provides clear step-by-step instructions from build to execution"
  artifacts:
    - path: "examples/README.md"
      provides: "Testing guide with build instructions and per-example walkthrough"
      min_lines: 80
    - path: "examples/01-hello-world/hello.wdl"
      provides: "Simplest possible workflow with string input/output"
    - path: "examples/02-scatter-gather/scatter.wdl"
      provides: "Scatter over array with gather output"
    - path: "examples/03-conditional/conditional.wdl"
      provides: "If-else conditional execution"
    - path: "examples/04-file-io/file_processing.wdl"
      provides: "File read/write with read_string, write_lines"
    - path: "examples/05-multi-step/pipeline.wdl"
      provides: "Multi-call workflow with task chaining"
  key_links: []
---

<objective>
Create an examples/ directory with sample WDL workflows, JSON input files, and a comprehensive testing guide (README.md) so users can build flowy and verify it works end-to-end.

Purpose: Provide runnable examples that demonstrate flowy features and serve as manual testing documentation.
Output: examples/ directory with 5 example subdirectories and a README.
</objective>

<execution_context>
@/home/uehara/.claude/get-shit-done/workflows/execute-plan.md
@/home/uehara/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@./CLAUDE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create sample WDL workflows and input files</name>
  <files>
    examples/01-hello-world/hello.wdl,
    examples/01-hello-world/inputs.json,
    examples/02-scatter-gather/scatter.wdl,
    examples/02-scatter-gather/inputs.json,
    examples/03-conditional/conditional.wdl,
    examples/03-conditional/inputs.json,
    examples/04-file-io/file_processing.wdl,
    examples/04-file-io/inputs.json,
    examples/05-multi-step/pipeline.wdl,
    examples/05-multi-step/inputs.json
  </files>
  <action>
Create 5 example directories under examples/, each with a .wdl file and inputs.json.

All WDL files must use `version 1.2` and `runtime { docker: "ubuntu:22.04" }` on every task.

**01-hello-world**: Minimal workflow. One task `say_hello` takes `String name`, runs `echo "Hello, ~{name}!"`, outputs `String greeting = read_string(stdout())`. Workflow calls it with default name "World". inputs.json: `{"hello.name": "flowy"}`.

**02-scatter-gather**: Workflow `scatter_example` takes `Array[String] samples`. Scatter over samples, call a task `process_sample` that runs `echo "Processing: ~{sample_name}" && echo "~{sample_name}_result"`, outputs result via read_string(stdout()). Workflow gathers scattered results into `Array[String] results`. inputs.json: `{"scatter_example.samples": ["A", "B", "C"]}`.

**03-conditional**: Workflow `conditional_example` takes `Boolean run_extended` and `String message`. Always calls a task `basic_report` that echoes the message. If run_extended is true, calls `extended_report` that echoes a longer version. Workflow outputs both (extended output is optional). inputs.json: `{"conditional_example.run_extended": true, "conditional_example.message": "Hello from flowy"}`.

**04-file-io**: Workflow `file_processing` takes `String content`. Task `write_data` writes content to a file using `echo "~{content}" > output.txt`, outputs `File result = "output.txt"`. Task `read_data` takes the File, runs `wc -l < ~{input_file} && cat ~{input_file}`, outputs line count and content via read_lines/read_string. inputs.json: `{"file_processing.content": "line1\nline2\nline3"}`.

**05-multi-step**: Workflow `pipeline` takes `String raw_data`. Three tasks chained: `step1_parse` echoes the raw_data, `step2_transform` uppercases input via `echo "~{data}" | tr '[:lower:]' '[:upper:]'`, `step3_format` wraps result in "=== ~{data} ===". Each outputs read_string(stdout()). Workflow chains call outputs. inputs.json: `{"pipeline.raw_data": "hello world"}`.

Ensure all input keys use `workflow_name.input_name` format in JSON files.
  </action>
  <verify>
    <automated>find examples/ -name "*.wdl" | wc -l | grep -q 5 && find examples/ -name "inputs.json" | wc -l | grep -q 5 && echo "OK"</automated>
  </verify>
  <done>5 example directories exist, each with a valid .wdl and inputs.json file</done>
</task>

<task type="auto">
  <name>Task 2: Create testing guide README</name>
  <files>examples/README.md</files>
  <action>
Create examples/README.md as a comprehensive testing guide. Structure:

**Title**: "flowy Examples and Testing Guide"

**Prerequisites section**: Go 1.21+, Docker running, git clone + `go build -o flowy ./cmd/flowy/`

**Quick start section**: Build command, verify with `./flowy --version`, run simplest example: `./flowy run examples/01-hello-world/hello.wdl`

**CLI Reference section**: Brief summary of both subcommands:
- `flowy check [--path <dir>] <file.wdl>` -- parse and type-check
- `flowy run [--input <file>] [--task <name>] [--dir <dir>] [--path <dir>] [--verbose] <file.wdl>` -- execute

**Examples section**: For each of the 5 examples, include:
- Feature demonstrated (e.g., "Scatter/Gather parallelism")
- Brief description of what the workflow does
- Run command without inputs (uses defaults if available)
- Run command with inputs: `./flowy run --input examples/XX/inputs.json examples/XX/workflow.wdl`
- Run with verbose: add `--verbose` flag
- Expected output description (what the user should see in outputs.json)
- How to check results: `cat <exec-dir>/outputs.json`

**Running individual tasks section**: Show `--task` flag usage with example 01: `./flowy run --task say_hello --input examples/01-hello-world/inputs.json examples/01-hello-world/hello.wdl`

**Troubleshooting section**: Common issues -- Docker not running, permission denied, "no workflow defined" (use --task), checking execution directory for stdout.txt/stderr.txt logs.

**Checking WDL syntax section**: Show `flowy check` on each example to verify parsing.

All paths should be relative to project root. Use code blocks for all commands. Keep explanations concise.
  </action>
  <verify>
    <automated>test -f examples/README.md && grep -q "Prerequisites" examples/README.md && grep -q "flowy run" examples/README.md && grep -q "Troubleshooting" examples/README.md && echo "OK"</automated>
  </verify>
  <done>README.md exists with build instructions, per-example walkthroughs, CLI reference, and troubleshooting section</done>
</task>

</tasks>

<verification>
- All 5 WDL files parse successfully: `for f in examples/*/*.wdl; do ./flowy check "$f" && echo "OK: $f"; done`
- README contains runnable commands for every example
- All JSON input files are valid JSON: `for f in examples/*/inputs.json; do python3 -c "import json; json.load(open('$f'))" && echo "OK: $f"; done`
</verification>

<success_criteria>
- 5 example directories under examples/, each with .wdl and inputs.json
- examples/README.md with build instructions, CLI reference, example walkthroughs, and troubleshooting
- All WDL files pass `flowy check`
- All JSON files are valid
</success_criteria>

<output>
After completion, create `.planning/quick/1-create-test-data-sample-workflow-and-tes/1-SUMMARY.md`
</output>
