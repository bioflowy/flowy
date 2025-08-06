package tree

import (
	"fmt"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
)

// WorkflowGraphBuilder helps build dependency graphs for workflow nodes
type WorkflowGraphBuilder struct {
	nodes   []WorkflowNode
	typeEnv *env.Bindings[types.Base]
}

// NewWorkflowGraphBuilder creates a new workflow graph builder
func NewWorkflowGraphBuilder(nodes []WorkflowNode, initialTypeEnv *env.Bindings[types.Base]) *WorkflowGraphBuilder {
	return &WorkflowGraphBuilder{
		nodes:   nodes,
		typeEnv: initialTypeEnv,
	}
}

// BuildDependencyGraph creates a dependency graph for the workflow nodes
func (wb *WorkflowGraphBuilder) BuildDependencyGraph() (map[string][]string, error) {
	dependencies := make(map[string][]string)
	currentTypeEnv := wb.typeEnv

	// Process nodes in order to build up the type environment
	for _, node := range wb.nodes {
		nodeName := node.WorkflowNodeName()

		// Get dependencies for this node
		deps, err := node.Dependencies(currentTypeEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependencies for node %s: %w", nodeName, err)
		}

		dependencies[nodeName] = deps

		// Update type environment with this node's contributions
		currentTypeEnv, err = node.AddToTypeEnv(currentTypeEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to update type environment for node %s: %w", nodeName, err)
		}
	}

	return dependencies, nil
}

// ValidateWorkflowGraph validates that the workflow dependency graph is acyclic
func ValidateWorkflowGraph(dependencies map[string][]string) error {
	// Implement cycle detection using DFS
	visiting := make(map[string]bool)
	visited := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		if visiting[node] {
			return fmt.Errorf("cycle detected in workflow dependencies involving node: %s", node)
		}
		if visited[node] {
			return nil
		}

		visiting[node] = true

		for _, dep := range dependencies[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[node] = false
		visited[node] = true
		return nil
	}

	for node := range dependencies {
		if !visited[node] {
			if err := visit(node); err != nil {
				return err
			}
		}
	}

	return nil
}

// TopologicalSort performs a topological sort of workflow nodes
func TopologicalSort(dependencies map[string][]string) ([]string, error) {
	// First validate the graph is acyclic
	if err := ValidateWorkflowGraph(dependencies); err != nil {
		return nil, err
	}

	// Calculate in-degrees
	inDegree := make(map[string]int)
	allNodes := make(map[string]bool)

	// Initialize in-degrees and collect all nodes
	for node, deps := range dependencies {
		allNodes[node] = true
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}

		for _, dep := range deps {
			allNodes[dep] = true
			inDegree[node]++
		}
	}

	// Find nodes with no incoming edges
	queue := make([]string, 0)
	for node := range allNodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]string, 0)

	// Process nodes in topological order
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for dependent nodes
		for node, deps := range dependencies {
			for _, dep := range deps {
				if dep == current {
					inDegree[node]--
					if inDegree[node] == 0 {
						queue = append(queue, node)
					}
				}
			}
		}
	}

	return result, nil
}

// ResolveWorkflowTypes resolves types for all workflow nodes
func ResolveWorkflowTypes(nodes []WorkflowNode, initialTypeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error) {
	currentTypeEnv := initialTypeEnv

	// Build dependency graph
	builder := NewWorkflowGraphBuilder(nodes, initialTypeEnv)
	dependencies, err := builder.BuildDependencyGraph()
	if err != nil {
		return nil, err
	}

	// Get topological order
	order, err := TopologicalSort(dependencies)
	if err != nil {
		return nil, err
	}

	// Create a map for fast node lookup
	nodeMap := make(map[string]WorkflowNode)
	for _, node := range nodes {
		nodeMap[node.WorkflowNodeName()] = node
	}

	// Process nodes in topological order
	for _, nodeName := range order {
		if node, exists := nodeMap[nodeName]; exists {
			currentTypeEnv, err = node.AddToTypeEnv(currentTypeEnv)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve types for node %s: %w", nodeName, err)
			}
		}
	}

	return currentTypeEnv, nil
}

// FindNodeByName finds a workflow node by name
func FindNodeByName(nodes []WorkflowNode, name string) WorkflowNode {
	for _, node := range nodes {
		if node.WorkflowNodeName() == name {
			return node
		}
	}
	return nil
}

// CollectAllDependencies recursively collects all dependencies for a node
func CollectAllDependencies(nodeName string, dependencies map[string][]string, visited map[string]bool) []string {
	if visited[nodeName] {
		return []string{}
	}

	visited[nodeName] = true
	depSet := make(map[string]bool)

	for _, dep := range dependencies[nodeName] {
		depSet[dep] = true
		transitiveDeps := CollectAllDependencies(dep, dependencies, visited)
		for _, transitiveDep := range transitiveDeps {
			depSet[transitiveDep] = true
		}
	}

	// Convert set to slice
	allDeps := make([]string, 0, len(depSet))
	for dep := range depSet {
		allDeps = append(allDeps, dep)
	}

	return allDeps
}

// ValidateDocumentStructure validates the overall structure of a WDL document
func ValidateDocumentStructure(doc *Document) error {
	if doc == nil {
		return errors.NewValidationErrorFromPos(errors.SourcePosition{}, "document cannot be nil")
	}

	// Check for duplicate task names
	taskNames := make(map[string]bool)
	for _, task := range doc.Tasks {
		if taskNames[task.Name] {
			return &errors.MultipleDefinitions{
				ValidationError: errors.NewValidationErrorFromPos(task.Pos, fmt.Sprintf("duplicate task name: %s", task.Name)),
			}
		}
		taskNames[task.Name] = true
	}

	// Check for duplicate struct names
	structNames := make(map[string]bool)
	for _, structDef := range doc.Structs {
		if structNames[structDef.Name] {
			return &errors.MultipleDefinitions{
				ValidationError: errors.NewValidationErrorFromPos(structDef.Pos, fmt.Sprintf("duplicate struct name: %s", structDef.Name)),
			}
		}
		structNames[structDef.Name] = true
	}

	// Validate workflow if present
	if doc.Workflow != nil {
		if err := ValidateWorkflowStructure(doc.Workflow); err != nil {
			return err
		}
	}

	return nil
}

// ValidateWorkflowStructure validates the structure of a workflow
func ValidateWorkflowStructure(workflow *Workflow) error {
	if workflow == nil {
		return nil
	}

	// Build and validate dependency graph
	initialTypeEnv := env.NewBindings[types.Base]()

	// Add input declarations to initial type environment
	for _, input := range workflow.Inputs {
		initialTypeEnv = initialTypeEnv.Bind(input.Name, input.Type, &input.Pos)
	}

	builder := NewWorkflowGraphBuilder(workflow.Body, initialTypeEnv)
	dependencies, err := builder.BuildDependencyGraph()
	if err != nil {
		return err
	}

	// Validate the dependency graph is acyclic
	return ValidateWorkflowGraph(dependencies)
}
