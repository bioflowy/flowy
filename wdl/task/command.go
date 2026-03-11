package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/values"
)

// BuildCommand expands the task's command template using container-translated env
// values, writes the resulting script to execDir/command, and returns the path to
// the command file.
//
// Steps:
//  1. Evaluate private declarations (task.Decls) into the env (these may be
//     referenced by the command expression).
//  2. Build a translated env by replacing all FileValue/DirectoryValue host paths
//     with container paths via pm.TranslateValue.
//  3. Evaluate task.Command via eval.Eval with the translated env.
//  4. Write the expanded command string to filepath.Join(execDir, "command") mode 0644.
//  5. Return the path to the written file.
func BuildCommand(task *ast.Task, env *eval.Env, pm *PathMap, execDir string, workDir string, structDefs ...map[string]*ast.StructDef) (string, error) {
	// Private declarations are already evaluated into env by RunTask before
	// BuildCommand is called. Build a translated env with container paths.
	// Step 1: build a translated env with container paths substituted for host paths.
	translatedEnv := translateEnv(env, pm)

	// Step 3: evaluate the command expression using the translated env.
	// Pass workDir so write_* functions create files inside the task work directory
	// (which is bind-mounted into the container as /mnt/outputs).
	cmdCtx := eval.ExecContext{WorkDir: workDir}
	if len(structDefs) > 0 {
		cmdCtx.StructDefs = structDefs[0]
	}
	v, err := eval.Eval(task.Command, translatedEnv, cmdCtx)
	if err != nil {
		return "", fmt.Errorf("evaluating command for task %q: %w", task.Name, err)
	}

	sv, ok := v.(values.StringValue)
	if !ok {
		return "", fmt.Errorf("command expression must evaluate to String, got %T", v)
	}

	// Step 4: translate any host workDir paths in the command to container paths.
	// Files created by write_* during command evaluation are placed in workDir,
	// but the container sees workDir as /mnt/outputs.
	cmd := sv.Value
	if workDir != "" {
		cmd = strings.ReplaceAll(cmd, workDir+"/", "/mnt/outputs/")
		cmd = strings.ReplaceAll(cmd, workDir, "/mnt/outputs")
	}

	// Step 5: write the expanded command string to execDir/command.
	scriptPath := filepath.Join(execDir, "command")
	if err := os.WriteFile(scriptPath, []byte(cmd), 0644); err != nil {
		return "", fmt.Errorf("writing command script for task %q: %w", task.Name, err)
	}

	// Step 6: return path to the written file.
	return scriptPath, nil
}

// translateEnv creates a new eval.Env that contains the same bindings as env
// (current scope only) but with all FileValue/DirectoryValue host paths replaced
// by container paths via pm.TranslateValue. The parent chain is preserved.
//
// This function calls env.ForEach to iterate the current scope's bindings without
// exposing the underlying map. Only the innermost scope is translated; parent
// scopes are copied by reference (they should not contain task-local File paths).
func translateEnv(env *eval.Env, pm *PathMap) *eval.Env {
	// Build a new env with the same parent chain.
	// We preserve the parent reference so variable resolution still works for
	// outer scopes (workflow variables etc.), but translate only the current scope.
	newEnv := eval.NewEnv(env.Parent())
	env.ForEach(func(name string, v values.Value) {
		newEnv.Define(name, pm.TranslateValue(v))
	})
	return newEnv
}
