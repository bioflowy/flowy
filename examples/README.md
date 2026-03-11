# flowy Examples and Testing Guide

This directory contains sample WDL workflows that demonstrate flowy features
and serve as end-to-end tests. Each example includes a `.wdl` workflow file
and a `inputs.json` file with sample inputs.

## Prerequisites

- **Go 1.21+** installed
- **Docker** installed and running (`docker info` should succeed)
- Clone the repository and build:

```bash
git clone https://github.com/bioflowy/flowy.git
cd flowy
go build -o flowy ./cmd/flowy/
```

## Quick Start

Verify the build:

```bash
./flowy --version
```

Run the simplest example:

```bash
./flowy run examples/01-hello-world/hello.wdl
```

Run with custom inputs:

```bash
./flowy run --input examples/01-hello-world/inputs.json examples/01-hello-world/hello.wdl
```

## CLI Reference

flowy provides two subcommands:

### `flowy check` -- Parse and Type-Check

```bash
flowy check [--path <dir>] <file.wdl>
```

Parses the WDL file and runs static type checking. Reports errors in
rustc-style format with source line context. Use `--path` to specify
additional search directories for imports.

### `flowy run` -- Execute Workflow

```bash
flowy run [--input <file>] [--task <name>] [--dir <dir>] [--path <dir>] [--verbose] <file.wdl>
```

Executes the workflow (or a single task with `--task`). Options:

| Flag | Description |
|------|-------------|
| `--input <file>` | JSON file with input values |
| `--task <name>` | Run a single task instead of the workflow |
| `--dir <dir>` | Execution directory (default: auto-generated) |
| `--path <dir>` | Additional search path for WDL imports |
| `--verbose` | Print task stdout/stderr to console |

## Examples

### 01 -- Hello World

**Feature:** Minimal workflow with string input and output.

A single task `say_hello` takes a name, prints a greeting, and captures
stdout as output.

```bash
# Run with default input (name = "World")
./flowy run examples/01-hello-world/hello.wdl

# Run with custom input (name = "flowy")
./flowy run --input examples/01-hello-world/inputs.json examples/01-hello-world/hello.wdl

# Run with verbose output
./flowy run --verbose --input examples/01-hello-world/inputs.json examples/01-hello-world/hello.wdl
```

**Expected output:** The execution directory will contain `outputs.json` with
a greeting string like `"Hello, flowy!"`.

```bash
cat <exec-dir>/outputs.json
```

### 02 -- Scatter/Gather

**Feature:** Scatter over an array with gathered results.

The workflow takes an array of sample names, scatters over them to process
each in parallel, and gathers the results.

```bash
# Run with inputs (samples = ["A", "B", "C"])
./flowy run --input examples/02-scatter-gather/inputs.json examples/02-scatter-gather/scatter.wdl

# Run with verbose output
./flowy run --verbose --input examples/02-scatter-gather/inputs.json examples/02-scatter-gather/scatter.wdl
```

**Expected output:** Each sample is processed in its own container. The
`outputs.json` will contain results from each scatter shard.

```bash
cat <exec-dir>/outputs.json
```

### 03 -- Conditional Execution

**Feature:** If-else conditional task execution.

The workflow always runs a `basic_report` task. If `run_extended` is true,
it also runs an `extended_report` task.

```bash
# Run with extended report enabled
./flowy run --input examples/03-conditional/inputs.json examples/03-conditional/conditional.wdl

# Run with verbose output
./flowy run --verbose --input examples/03-conditional/inputs.json examples/03-conditional/conditional.wdl
```

**Expected output:** Both `basic_report` and `extended_report` tasks run.
The `outputs.json` will contain both report outputs. Set `run_extended` to
`false` in the JSON to skip the extended report.

```bash
cat <exec-dir>/outputs.json
```

### 04 -- File I/O

**Feature:** File read/write operations between tasks.

The workflow writes content to a file in one task, then passes that file to
a second task that reads it and reports line count and content.

```bash
# Run with multi-line content
./flowy run --input examples/04-file-io/inputs.json examples/04-file-io/file_processing.wdl

# Run with verbose output
./flowy run --verbose --input examples/04-file-io/inputs.json examples/04-file-io/file_processing.wdl
```

**Expected output:** The `write_data` task creates a file, and `read_data`
reads it back. The `outputs.json` will contain the file content and line count.

```bash
cat <exec-dir>/outputs.json
```

### 05 -- Multi-Step Pipeline

**Feature:** Multi-task workflow with chained outputs.

Three tasks execute in sequence: `step1_parse` echoes raw data,
`step2_transform` converts it to uppercase, and `step3_format` wraps
the result in a formatted string.

```bash
# Run the pipeline (raw_data = "hello world")
./flowy run --input examples/05-multi-step/inputs.json examples/05-multi-step/pipeline.wdl

# Run with verbose output
./flowy run --verbose --input examples/05-multi-step/inputs.json examples/05-multi-step/pipeline.wdl
```

**Expected output:** The pipeline transforms `"hello world"` through three
stages. The final `outputs.json` will contain `"=== HELLO WORLD ==="`.

```bash
cat <exec-dir>/outputs.json
```

## Running Individual Tasks

You can run a single task from a WDL file using the `--task` flag. This is
useful for testing individual tasks without running the full workflow.

```bash
# Run just the say_hello task
./flowy run --task say_hello --input examples/01-hello-world/inputs.json examples/01-hello-world/hello.wdl

# Run just step2_transform with custom input
./flowy run --task step2_transform examples/05-multi-step/pipeline.wdl
```

When using `--task`, only that task's inputs are required in the JSON file.

## Checking WDL Syntax

Use `flowy check` to validate WDL files without executing them:

```bash
# Check all examples
for f in examples/*/*.wdl; do
  ./flowy check "$f" && echo "OK: $f"
done
```

```bash
# Check a single file
./flowy check examples/01-hello-world/hello.wdl
```

## Troubleshooting

### Docker not running

```
Cannot connect to the Docker daemon
```

Start Docker and verify with `docker info`.

### Permission denied

```
Got permission denied while trying to connect to the Docker daemon socket
```

Add your user to the `docker` group: `sudo usermod -aG docker $USER`, then
log out and back in.

### "no workflow defined"

If a WDL file has no `workflow` block, you must specify a task with `--task`:

```bash
./flowy run --task <task_name> file.wdl
```

### Checking execution logs

Each task execution creates a directory with `stdout.txt` and `stderr.txt`.
Check these files for debugging:

```bash
# Find the execution directory (printed at the start of execution)
ls <exec-dir>/
cat <exec-dir>/stdout.txt
cat <exec-dir>/stderr.txt
```

### Invalid JSON inputs

Validate your input JSON before running:

```bash
python3 -c "import json; json.load(open('inputs.json'))"
```

Input keys must use the format `workflow_name.input_name` (e.g.,
`"hello.name": "flowy"`).
