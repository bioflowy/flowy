package workflow

import "github.com/bioflowy/flowy/wdl/ast"

// dagNode represents a single execution unit in the workflow DAG.
// For top-level calls, id is the call alias (or last element of callee).
// For top-level scatter/conditional blocks, id is a synthetic "scatter_N"/"if_N" id.
type dagNode struct {
	id         string
	node       ast.WorkflowNode
	deps       []string // IDs of nodes this node depends on
	successors []string // IDs of nodes that depend on this node
	indegree   int      // number of incoming edges (used by Kahn's algorithm)
}

// buildDAG constructs a DAG from the top-level workflow body elements.
// Each ast.Call becomes a DAG node (id = alias or last callee element).
// Each ast.Scatter and ast.Conditional becomes a DAG node with a synthetic id.
// ast.Decl nodes are skipped (evaluated synchronously by the scheduler).
//
// Edges are created from:
//  1. Data-flow: when a call's input expression references another call's id
//     (IdentExpr matching a known call id, or GetExpr whose Object matches)
//  2. Explicit ordering: call.After[i] lists that must complete before this call
func buildDAG(body []ast.WorkflowNode) (map[string]*dagNode, error) {
	nodes := make(map[string]*dagNode)
	scatter_n := 0
	if_n := 0

	// First pass: register all top-level nodes to build the knownCalls set.
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Call:
			id := callID(n)
			nodes[id] = &dagNode{id: id, node: n}
		case *ast.Scatter:
			scatter_n++
			id := scatterID(n, scatter_n)
			nodes[id] = &dagNode{id: id, node: n}
		case *ast.Conditional:
			if_n++
			id := conditionalID(n, if_n)
			nodes[id] = &dagNode{id: id, node: n}
		case *ast.Decl:
			id := "decl_" + n.Name
			nodes[id] = &dagNode{id: id, node: n}
		}
	}

	// knownNodes includes call aliases, scatter/if IDs, and decl_ prefixed names.
	knownNodes := make(map[string]bool, len(nodes))
	for id := range nodes {
		knownNodes[id] = true
	}

	// varToNodeID maps variable names produced by any DAG node to that node's ID.
	// This includes: call aliases, decl variable names, and names produced by scatter/conditional bodies.
	varToNodeID := make(map[string]string)
	scatter_n2, if_n2 := 0, 0
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Call:
			varToNodeID[callID(n)] = callID(n)
		case *ast.Decl:
			varToNodeID[n.Name] = "decl_" + n.Name
		case *ast.Scatter:
			scatter_n2++
			id := scatterID(n, scatter_n2)
			collectBodyOutputNames(n.Body, id, varToNodeID)
		case *ast.Conditional:
			if_n2++
			id := conditionalID(n, if_n2)
			collectBodyOutputNames(n.Body, id, varToNodeID)
		}
	}

	// Second pass: build edges.
	scatter_n = 0
	if_n = 0
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Call:
			id := callID(n)
			node := nodes[id]

			// Data-flow edges from input expressions.
			deps := collectExprDeps2(n.Inputs, knownNodes, varToNodeID)

			// Explicit after edges.
			for _, afterID := range n.After {
				if knownNodes[afterID] {
					deps = append(deps, afterID)
				}
			}

			// Deduplicate and record edges.
			deps = uniqueStrings(deps)
			node.deps = deps
			for _, depID := range deps {
				depNode := nodes[depID]
				depNode.successors = append(depNode.successors, id)
				node.indegree++
			}

		case *ast.Scatter:
			scatter_n++
			id := scatterID(n, scatter_n)
			node := nodes[id]

			// Scatter depends on calls referenced in its Expr and inner call inputs.
			deps := collectExprDepsNode(n.Expr, knownNodes, varToNodeID)
			deps = append(deps, collectBodyDeps2(n.Body, knownNodes, varToNodeID)...)
			deps = removeString(uniqueStrings(deps), id) // filter self-dependency
			node.deps = deps
			for _, depID := range deps {
				depNode := nodes[depID]
				depNode.successors = append(depNode.successors, id)
				node.indegree++
			}

		case *ast.Conditional:
			if_n++
			id := conditionalID(n, if_n)
			node := nodes[id]

			// Conditional depends on calls referenced in its Expr and inner call inputs.
			deps := collectExprDepsNode(n.Expr, knownNodes, varToNodeID)
			deps = append(deps, collectBodyDeps2(n.Body, knownNodes, varToNodeID)...)
			deps = removeString(uniqueStrings(deps), id) // filter self-dependency
			node.deps = deps
			for _, depID := range deps {
				depNode := nodes[depID]
				depNode.successors = append(depNode.successors, id)
				node.indegree++
			}

		case *ast.Decl:
			id := "decl_" + n.Name
			node := nodes[id]

			// Decl depends on calls/decls referenced in its expression.
			deps := collectExprDepsNode(n.Expr, knownNodes, varToNodeID)
			deps = uniqueStrings(deps)
			node.deps = deps
			for _, depID := range deps {
				depNode := nodes[depID]
				depNode.successors = append(depNode.successors, id)
				node.indegree++
			}
		}
	}

	return nodes, nil
}

// topoSort performs Kahn's algorithm on the DAG nodes.
// Returns a topologically ordered list of node IDs.
// Returns an error if a cycle is detected (len(order) != len(nodes)).
func topoSort(nodes map[string]*dagNode) ([]string, error) {
	// Build a working copy of indegrees to avoid mutating the original.
	indegree := make(map[string]int, len(nodes))
	for id, n := range nodes {
		indegree[id] = n.indegree
	}

	// Seed queue with all indegree-0 nodes (deterministic order by id).
	queue := make([]string, 0, len(nodes))
	for id, deg := range indegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sortStrings(queue)

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		// Pop front.
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)

		// Decrease indegree of successors; add newly-zero ones to queue.
		for _, succID := range nodes[cur].successors {
			indegree[succID]--
			if indegree[succID] == 0 {
				queue = append(queue, succID)
				sortStrings(queue)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, &CycleError{Order: order, Total: len(nodes)}
	}
	return order, nil
}

// CycleError is returned by topoSort when a cycle is detected in the DAG.
type CycleError struct {
	Order []string // partial order (nodes not in cycle)
	Total int      // total number of nodes
}

func (e *CycleError) Error() string {
	return "workflow dependency graph contains a cycle"
}

// collectExprDeps2 walks call input expressions looking for references to known DAG nodes.
func collectExprDeps2(inputs map[string]ast.Expr, knownNodes map[string]bool, declVarToID map[string]string) []string {
	var deps []string
	for _, expr := range inputs {
		deps = append(deps, collectExprDepsNode(expr, knownNodes, declVarToID)...)
	}
	return deps
}

// collectExprDepsNode recursively walks an expression tree looking for references
// to known DAG node IDs (call aliases, scatter/if IDs, or decl variable names).
func collectExprDepsNode(expr ast.Expr, knownNodes map[string]bool, declVarToID map[string]string) []string {
	if expr == nil {
		return nil
	}
	var deps []string
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if knownNodes[e.Name] {
			deps = append(deps, e.Name)
		} else if id, ok := declVarToID[e.Name]; ok {
			deps = append(deps, id)
		}
	case *ast.GetExpr:
		// Most common: call_name.output_name
		if obj, ok := e.Object.(*ast.IdentExpr); ok {
			if knownNodes[obj.Name] {
				deps = append(deps, obj.Name)
			} else if id, ok := declVarToID[obj.Name]; ok {
				deps = append(deps, id)
			}
		} else {
			// Recurse in case it is a deeper expression.
			deps = append(deps, collectExprDepsNode(e.Object, knownNodes, declVarToID)...)
		}
	case *ast.BinaryExpr:
		deps = append(deps, collectExprDepsNode(e.Left, knownNodes, declVarToID)...)
		deps = append(deps, collectExprDepsNode(e.Right, knownNodes, declVarToID)...)
	case *ast.UnaryExpr:
		deps = append(deps, collectExprDepsNode(e.Operand, knownNodes, declVarToID)...)
	case *ast.IndexExpr:
		deps = append(deps, collectExprDepsNode(e.Collection, knownNodes, declVarToID)...)
		deps = append(deps, collectExprDepsNode(e.Index, knownNodes, declVarToID)...)
	case *ast.IfThenElse:
		deps = append(deps, collectExprDepsNode(e.Cond, knownNodes, declVarToID)...)
		deps = append(deps, collectExprDepsNode(e.Then, knownNodes, declVarToID)...)
		deps = append(deps, collectExprDepsNode(e.Else, knownNodes, declVarToID)...)
	case *ast.ApplyExpr:
		for _, arg := range e.Args {
			deps = append(deps, collectExprDepsNode(arg, knownNodes, declVarToID)...)
		}
	case *ast.ArrayExpr:
		for _, item := range e.Items {
			deps = append(deps, collectExprDepsNode(item, knownNodes, declVarToID)...)
		}
	case *ast.MapExpr:
		for _, item := range e.Items {
			deps = append(deps, collectExprDepsNode(item.Key, knownNodes, declVarToID)...)
			deps = append(deps, collectExprDepsNode(item.Value, knownNodes, declVarToID)...)
		}
	case *ast.PairExpr:
		deps = append(deps, collectExprDepsNode(e.Left, knownNodes, declVarToID)...)
		deps = append(deps, collectExprDepsNode(e.Right, knownNodes, declVarToID)...)
	}
	return deps
}

// collectBodyDeps2 walks a workflow body (nested calls in scatter/conditional) and
// collects deps from their input expressions, but does NOT treat nested calls as
// DAG nodes themselves — they are owned by their enclosing scatter/conditional.
func collectBodyDeps2(body []ast.WorkflowNode, knownNodes map[string]bool, varMap map[string]string) []string {
	var deps []string
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Call:
			deps = append(deps, collectExprDeps2(n.Inputs, knownNodes, varMap)...)
			for _, after := range n.After {
				if knownNodes[after] {
					deps = append(deps, after)
				}
			}
		case *ast.Scatter:
			deps = append(deps, collectExprDepsNode(n.Expr, knownNodes, varMap)...)
			deps = append(deps, collectBodyDeps2(n.Body, knownNodes, varMap)...)
		case *ast.Conditional:
			deps = append(deps, collectExprDepsNode(n.Expr, knownNodes, varMap)...)
			deps = append(deps, collectBodyDeps2(n.Body, knownNodes, varMap)...)
		case *ast.Decl:
			if n.Expr != nil {
				deps = append(deps, collectExprDepsNode(n.Expr, knownNodes, varMap)...)
			}
		}
	}
	return deps
}

// collectBodyOutputNames recursively collects all variable names produced by a
// scatter/conditional body and maps them to the owning DAG node ID.
func collectBodyOutputNames(body []ast.WorkflowNode, ownerID string, varToNodeID map[string]string) {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.Call:
			varToNodeID[callID(n)] = ownerID
		case *ast.Decl:
			varToNodeID[n.Name] = ownerID
		case *ast.Scatter:
			collectBodyOutputNames(n.Body, ownerID, varToNodeID)
		case *ast.Conditional:
			collectBodyOutputNames(n.Body, ownerID, varToNodeID)
		}
	}
}

// callID returns the node identifier for a call: alias if set, else last element of Callee.
func callID(c *ast.Call) string {
	if c.Alias != "" {
		return c.Alias
	}
	return c.Callee[len(c.Callee)-1]
}

// scatterID returns a synthetic id for a scatter node.
func scatterID(_ *ast.Scatter, n int) string {
	return "scatter_" + itoa(n)
}

// conditionalID returns a synthetic id for a conditional node.
func conditionalID(_ *ast.Conditional, n int) string {
	return "if_" + itoa(n)
}

// itoa converts an int to its decimal string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// uniqueStrings returns a new slice with duplicate strings removed (preserving first occurrence).
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// removeString returns a new slice with all occurrences of s removed.
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

// sortStrings sorts a slice of strings in-place (insertion sort for small slices).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
