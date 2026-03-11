// Package checker implements the WDL 1.2 type checker.
// It traverses the AST produced by the parser and verifies type correctness.
package checker

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
)

// TypeError reports a type-checking error at a specific source location.
type TypeError struct {
	Pos     ast.Pos
	Message string
}

// Error implements the error interface.
func (e *TypeError) Error() string {
	return fmt.Sprintf("type error: %s\n  --> %s:%d:%d", e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}

// TypeEnv maps declaration names to their resolved WDL types.
// The top-level TypeEnv contains all declarations visible at document scope.
type TypeEnv map[string]types.Type

// Scope is a linked-list node representing a lexical scope.
// Each Scope holds its own declarations and a pointer to the enclosing parent scope.
type Scope struct {
	decls  map[string]types.Type
	parent *Scope
}

// newScope creates a new Scope whose parent is p (may be nil for the root scope).
func newScope(p *Scope) *Scope {
	return &Scope{
		decls:  make(map[string]types.Type),
		parent: p,
	}
}

// lookup searches for name in this scope and all enclosing scopes.
// Returns the type and true if found, or nil and false otherwise.
func (s *Scope) lookup(name string) (types.Type, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if t, ok := cur.decls[name]; ok {
			return t, true
		}
	}
	return nil, false
}

// define binds name to t in the current (innermost) scope.
func (s *Scope) define(name string, t types.Type) {
	s.decls[name] = t
}

// Check performs two-pass type checking on doc and returns:
//   - TypeEnv: all top-level declarations (merged from all tasks and workflows)
//   - []error: all type errors found (does not stop at first error)
//
// Pass 1 (collectDecls): registers every declaration into its scope with its
// declared type, applying arrayize/optionalize for scatter/if bodies.
// Pass 1 also builds structDefs from doc.Structs for use in struct validation.
// Pass 2 (validateDecls): runs Kahn's toposort per scope for cycle detection,
// then validates literal and ident initializer types using Coerces().
func Check(doc *ast.Document) (TypeEnv, []error) {
	env := make(TypeEnv)
	var errs []error

	// Pass 1: build structDefs map from the document's struct definitions.
	// This map is passed to task/workflow checking for struct literal and field access validation.
	structDefs := make(map[string]*ast.StructDef, len(doc.Structs))
	for _, s := range doc.Structs {
		structDefs[s.Name] = s
	}

	// Process each task independently.
	for _, task := range doc.Tasks {
		scope, taskErrs := checkTask(task, structDefs)
		errs = append(errs, taskErrs...)
		// Merge task declarations into the top-level env.
		for name, t := range scope.decls {
			env[name] = t
		}
	}

	// Process each workflow independently.
	for _, wf := range doc.Workflows {
		scope, wfErrs := checkWorkflow(wf, structDefs, doc.Tasks)
		errs = append(errs, wfErrs...)
		// Merge workflow declarations into the top-level env.
		for name, t := range scope.decls {
			env[name] = t
		}
	}

	return env, errs
}

// checkTask runs both passes for a single task and returns the resulting scope
// plus any errors found. structDefs is the document-level struct definition map.
func checkTask(task *ast.Task, structDefs map[string]*ast.StructDef) (*Scope, []error) {
	scope := newScope(nil)

	// Pass 1: collect all declarations (inputs, private decls, outputs).
	allDecls := make([]*ast.Decl, 0, len(task.Inputs)+len(task.Decls)+len(task.Outputs))
	allDecls = append(allDecls, task.Inputs...)
	allDecls = append(allDecls, task.Decls...)
	allDecls = append(allDecls, task.Outputs...)

	for _, d := range allDecls {
		scope.define(d.Name, d.Type)
	}

	// Build groups: all task decls are validated against the same flat scope.
	groups := make([]declGroup, len(allDecls))
	for i, d := range allDecls {
		groups[i] = declGroup{d, scope}
	}

	// Pass 2: validate declarations in topological order.
	errs := validateGroups(scope, groups, structDefs)
	return scope, errs
}

// declGroup bundles a declaration with the scope that was active when it was
// parsed. This is needed for validation because scatter-body declarations must
// be validated against an inner scope that includes the scatter loop variable,
// not the outer workflow scope.
type declGroup struct {
	decl  *ast.Decl
	scope *Scope // scope in which decl.Expr should be type-checked
}

// checkWorkflow runs both passes for a single workflow and returns the resulting
// scope plus any errors found. structDefs is the document-level struct definition map.
func checkWorkflow(wf *ast.Workflow, structDefs map[string]*ast.StructDef, tasks []*ast.Task) (*Scope, []error) {
	outerScope := newScope(nil)
	var groups []declGroup

	// Build task lookup map for resolving call outputs.
	taskMap := make(map[string]*ast.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.Name] = t
	}

	// Pass 1: register inputs.
	for _, d := range wf.Inputs {
		outerScope.define(d.Name, d.Type)
		groups = append(groups, declGroup{d, outerScope})
	}

	// Pass 1: walk body nodes, applying promotion for scatter/if.
	bodyGroups := collectWorkflowBodyGroups(wf.Body, outerScope, nil, taskMap)
	groups = append(groups, bodyGroups...)

	// Pass 1: register outputs.
	for _, d := range wf.Outputs {
		outerScope.define(d.Name, d.Type)
		groups = append(groups, declGroup{d, outerScope})
	}

	// Pass 2: validate all declarations in topological order.
	errs := validateGroups(outerScope, groups, structDefs)
	return outerScope, errs
}

// collectWorkflowBodyGroups walks a slice of WorkflowNode values and registers their
// declared names into outerScope. For Scatter nodes each inner name is
// registered as Array[T]; for Conditional nodes each inner name is registered
// as T?. The inner bodies are processed recursively so nesting composes
// correctly (scatter-inside-if → innermost promotion first, then outer wraps).
//
// promoteFn is the promotion function to apply on top of any promotions already
// applied by an inner scope. nil means no promotion (top-level body).
//
// Returns declGroup values pairing each decl with the scope it should be
// validated against (may be an inner scope containing scatter variables).
func collectWorkflowBodyGroups(nodes []ast.WorkflowNode, outerScope *Scope, promoteFn func(types.Type) types.Type, taskMap map[string]*ast.Task) []declGroup {
	var groups []declGroup

	for _, node := range nodes {
		switch n := node.(type) {
		case *ast.Decl:
			t := n.Type
			if promoteFn != nil {
				t = promoteFn(t)
			}
			outerScope.define(n.Name, t)
			groups = append(groups, declGroup{n, outerScope})

		case *ast.Call:
			registerCallOutputs(n, outerScope, promoteFn, taskMap)

		case *ast.Scatter:
			// Create an inner scope for the scatter body that includes the
			// loop variable. This lets body declarations reference the variable
			// by name during validation.
			scatterVarType := scatterVarElementType(n.Expr, outerScope)
			innerScope := newScope(outerScope)
			innerScope.define(n.Variable, scatterVarType)

			// Each name declared inside the scatter body gets arrayize() applied
			// on top of any outer promotion, but is registered in outerScope.
			scatterPromote := func(t types.Type) types.Type {
				if promoteFn != nil {
					t = promoteFn(t)
				}
				return arrayize(t)
			}
			inner := collectWorkflowBodyGroupsInner(n.Body, outerScope, innerScope, scatterPromote, taskMap)
			groups = append(groups, inner...)

		case *ast.Conditional:
			// Each name declared inside the if body gets optionalize() applied
			// on top of any outer promotion.
			condPromote := func(t types.Type) types.Type {
				if promoteFn != nil {
					t = promoteFn(t)
				}
				return optionalize(t)
			}
			inner := collectWorkflowBodyGroups(n.Body, outerScope, condPromote, taskMap)
			groups = append(groups, inner...)
		}
	}

	return groups
}

// collectWorkflowBodyGroupsInner is like collectWorkflowBodyGroups but uses
// innerScope for validation (so that scatter variables are in scope) while
// still registering promoted names into outerScope.
func collectWorkflowBodyGroupsInner(nodes []ast.WorkflowNode, outerScope *Scope, innerScope *Scope, promoteFn func(types.Type) types.Type, taskMap map[string]*ast.Task) []declGroup {
	var groups []declGroup

	for _, node := range nodes {
		switch n := node.(type) {
		case *ast.Decl:
			t := n.Type
			if promoteFn != nil {
				t = promoteFn(t)
			}
			outerScope.define(n.Name, t)
			// Validate against innerScope so scatter variable is accessible.
			groups = append(groups, declGroup{n, innerScope})

		case *ast.Call:
			registerCallOutputs(n, outerScope, promoteFn, taskMap)

		case *ast.Scatter:
			scatterVarType := scatterVarElementType(n.Expr, innerScope)
			nestedInner := newScope(innerScope)
			nestedInner.define(n.Variable, scatterVarType)

			scatterPromote := func(t types.Type) types.Type {
				if promoteFn != nil {
					t = promoteFn(t)
				}
				return arrayize(t)
			}
			inner := collectWorkflowBodyGroupsInner(n.Body, outerScope, nestedInner, scatterPromote, taskMap)
			groups = append(groups, inner...)

		case *ast.Conditional:
			condPromote := func(t types.Type) types.Type {
				if promoteFn != nil {
					t = promoteFn(t)
				}
				return optionalize(t)
			}
			inner := collectWorkflowBodyGroupsInner(n.Body, outerScope, innerScope, condPromote, taskMap)
			groups = append(groups, inner...)
		}
	}

	return groups
}

// registerCallOutputs resolves a Call node's target task and registers the call
// alias (or task name) as a StructType in the given scope. Each task output
// becomes a member of the struct, with promoteFn applied to its type.
func registerCallOutputs(call *ast.Call, scope *Scope, promoteFn func(types.Type) types.Type, taskMap map[string]*ast.Task) {
	taskName := call.Callee[len(call.Callee)-1]
	task, ok := taskMap[taskName]
	if !ok {
		return // unknown task — will be caught elsewhere
	}

	// Build StructType from task outputs, applying promoteFn to each member.
	members := make(map[string]types.Type, len(task.Outputs))
	for _, out := range task.Outputs {
		t := out.Type
		if promoteFn != nil {
			t = promoteFn(t)
		}
		members[out.Name] = t
	}

	// Call ID: alias if set, else task name.
	callID := call.Alias
	if callID == "" {
		callID = taskName
	}

	st := types.StructType{Name: callID, Members: members}
	scope.define(callID, st)
}

// validateGroups runs Kahn's topological sort on the declarations visible in
// outerScope, then validates each declaration's initializer type using the
// scope bundled in each declGroup (which may be an inner scope for scatter bodies).
// structDefs is passed through for struct literal and field access validation.
// All errors are collected; processing does not stop at the first error.
func validateGroups(outerScope *Scope, groups []declGroup, structDefs map[string]*ast.StructDef) []error {
	var errs []error

	// Build the dependency graph from ident references.
	// nodes: all declaration names in outerScope.
	// edges: name → list of names it depends on (ident refs found in its Expr that
	//         are also in outerScope.decls, i.e., peer declarations in the same container).
	nodes := make([]string, 0, len(groups))
	edges := make(map[string][]string, len(groups))
	groupByName := make(map[string]declGroup, len(groups))

	// First pass: collect all declaration names so we can distinguish
	// peer declarations from call aliases in the dependency check.
	for _, g := range groups {
		d := g.decl
		nodes = append(nodes, d.Name)
		groupByName[d.Name] = g
	}

	// Second pass: build dependency edges.
	for _, g := range groups {
		d := g.decl
		var deps []string
		if d.Expr != nil {
			for _, ident := range collectIdents(d.Expr) {
				// Only add dependency if the referenced name is a peer declaration
				// (registered in outerScope.decls directly — not inherited from parent)
				// AND is an actual declaration node (not a call alias registered as StructType).
				// Skip if the ident is shadowed by the group's own inner scope
				// (e.g. a scatter variable), which takes precedence.
				if _, inScope := outerScope.decls[ident]; inScope {
					if _, isDecl := groupByName[ident]; isDecl {
						// If the group's scope is different from outerScope and
						// defines this ident directly (e.g. scatter variable),
						// the ident refers to the inner binding, not the peer decl.
						if g.scope != outerScope {
							if _, inInner := g.scope.decls[ident]; inInner {
								continue
							}
						}
						deps = append(deps, ident)
					}
				}
			}
		}
		edges[d.Name] = deps
	}

	// Topological sort to detect cycles and establish evaluation order.
	order, err := toposort(nodes, edges)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	// Validate each declaration in topological order.
	for _, name := range order {
		g, ok := groupByName[name]
		if !ok {
			continue
		}
		if g.decl.Expr == nil {
			continue // unbound declaration (e.g., input stub), no initializer to check
		}
		exprErrs := checkExprType(g.decl, g.scope, structDefs)
		errs = append(errs, exprErrs...)
	}

	return errs
}

// scatterVarElementType derives the element type of the collection expression
// in a Scatter node. If the expression is an IdentExpr and that ident resolves
// to an ArrayType in scope, the item type is returned. For known stdlib
// functions like zip/cross, the return element type is inferred.
// Otherwise AnyType is returned as a safe fallback (no false type errors).
func scatterVarElementType(expr ast.Expr, scope *Scope) types.Type {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if t, found := scope.lookup(e.Name); found {
			if arr, ok := t.(types.ArrayType); ok {
				return arr.ItemType
			}
		}
	case *ast.ApplyExpr:
		// Infer element types for known stdlib functions that return arrays.
		switch e.Name {
		case "zip", "cross":
			// zip(Array[X], Array[Y]) -> Array[Pair[X,Y]]
			// cross(Array[X], Array[Y]) -> Array[Pair[X,Y]]
			if len(e.Args) >= 2 {
				leftType := inferExprArrayItemType(e.Args[0], scope)
				rightType := inferExprArrayItemType(e.Args[1], scope)
				return types.PairType{LeftType: leftType, RightType: rightType}
			}
		case "flatten":
			// flatten(Array[Array[X]]) -> Array[X]
			if len(e.Args) >= 1 {
				outerItem := inferExprArrayItemType(e.Args[0], scope)
				if arr, ok := outerItem.(types.ArrayType); ok {
					return arr.ItemType
				}
			}
		case "range":
			return types.IntType{}
		case "as_pairs":
			// as_pairs(Map[K,V]) -> Array[Pair[K,V]]
			if len(e.Args) >= 1 {
				if mapType := inferExprMapType(e.Args[0], scope); mapType != nil {
					return types.PairType{LeftType: mapType.KeyType, RightType: mapType.ValType}
				}
			}
		case "keys":
			// keys(Map[K,V]) -> Array[K]
			if len(e.Args) >= 1 {
				if mapType := inferExprMapType(e.Args[0], scope); mapType != nil {
					return mapType.KeyType
				}
			}
		case "values":
			// values(Map[K,V]) -> Array[V]
			if len(e.Args) >= 1 {
				if mapType := inferExprMapType(e.Args[0], scope); mapType != nil {
					return mapType.ValType
				}
			}
		}
	}
	return types.AnyType{}
}

// inferExprMapType attempts to infer the MapType of an expression.
// Returns the MapType if the expression resolves to a MapType, otherwise nil.
func inferExprMapType(expr ast.Expr, scope *Scope) *types.MapType {
	if ident, ok := expr.(*ast.IdentExpr); ok {
		if t, found := scope.lookup(ident.Name); found {
			if mt, ok := t.(types.MapType); ok {
				return &mt
			}
		}
	}
	return nil
}

// inferExprArrayItemType attempts to infer the item type of an Array expression.
// Returns the item type if the expression resolves to an ArrayType, otherwise AnyType.
func inferExprArrayItemType(expr ast.Expr, scope *Scope) types.Type {
	if ident, ok := expr.(*ast.IdentExpr); ok {
		if t, found := scope.lookup(ident.Name); found {
			if arr, ok := t.(types.ArrayType); ok {
				return arr.ItemType
			}
		}
	}
	return types.AnyType{}
}

// checkExprType checks whether decl's initializer expression is compatible with
// decl.Type. Only literal types, IdentExpr, StructExpr, and GetExpr are
// validated; all other expression types (BinaryExpr, ApplyExpr, etc.) are
// skipped per the plan. structDefs is used for struct literal and field access validation.
func checkExprType(decl *ast.Decl, scope *Scope, structDefs map[string]*ast.StructDef) []error {
	var errs []error

	var fromType types.Type

	switch e := decl.Expr.(type) {
	case *ast.BoolLit:
		fromType = types.BooleanType{}
	case *ast.IntLit:
		fromType = types.IntType{}
	case *ast.FloatLit:
		fromType = types.FloatType{}
	case *ast.NullLit:
		fromType = types.AnyType{Optional: true}
	case *ast.StringExpr:
		fromType = types.StringType{}
	case *ast.ArrayExpr:
		// Check non-empty constraint: Array[T]+ cannot be assigned an empty array literal.
		if at, ok := decl.Type.(types.ArrayType); ok && at.Nonempty && len(e.Items) == 0 {
			errs = append(errs, &TypeError{
				Pos:     decl.Pos,
				Message: fmt.Sprintf("cannot assign empty array to non-empty type %s", decl.Type.TypeName()),
			})
			return errs
		}
		return nil // skip further type checking for array literals
	case *ast.IdentExpr:
		t, ok := scope.lookup(e.Name)
		if !ok {
			errs = append(errs, &TypeError{
				Pos:     decl.Pos,
				Message: fmt.Sprintf("undefined name %q", e.Name),
			})
			return errs
		}
		fromType = t
	case *ast.StructExpr:
		structErrs, t := checkStructExpr(e, decl.Pos, scope, structDefs)
		errs = append(errs, structErrs...)
		if len(structErrs) > 0 {
			return errs
		}
		fromType = t
	case *ast.GetExpr:
		getErr, t := checkGetExpr(e, decl.Pos, scope, structDefs)
		if getErr != nil {
			errs = append(errs, getErr)
			return errs
		}
		fromType = t
	default:
		// Complex expressions (BinaryExpr, ApplyExpr, etc.) are deferred to Phase 4.
		return nil
	}

	// Check that fromType is assignable to decl.Type.
	if fromType.TypeName() != decl.Type.TypeName() && !Coerces(fromType, decl.Type) {
		errs = append(errs, &TypeError{
			Pos:     decl.Pos,
			Message: fmt.Sprintf("cannot use %s where %s is expected", fromType.TypeName(), decl.Type.TypeName()),
		})
	}

	return errs
}

// toposort implements Kahn's algorithm for topological ordering of nodes
// given a dependency edge map (name → list of names it depends on).
// Returns the ordered node names, or an error describing any cycle found.
func toposort(nodes []string, edges map[string][]string) ([]string, error) {
	// Compute in-degree for each node (number of nodes that depend on it).
	// Note: edges[a] = [b, c] means a depends on b and c, i.e., b and c must
	// come before a. So each entry in edges[a] is an outgoing edge FROM b/c TO a
	// in the "must-come-before" graph. We need to count how many prerequisites
	// each node has (its in-degree in the dependency graph).
	inDegree := make(map[string]int, len(nodes))
	// dependents maps each node to the list of nodes that depend on it.
	dependents := make(map[string][]string, len(nodes))

	for _, n := range nodes {
		if _, ok := inDegree[n]; !ok {
			inDegree[n] = 0
		}
	}

	for _, n := range nodes {
		for _, dep := range edges[n] {
			// n depends on dep → dep must come first → n's in-degree increases.
			inDegree[n]++
			dependents[dep] = append(dependents[dep], n)
		}
	}

	// Initialize queue with zero in-degree nodes.
	queue := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		// Pop from queue.
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)

		// Reduce in-degree for all nodes that depend on cur.
		for _, dependent := range dependents[cur] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(nodes) {
		// Collect names still in cycle (those with in-degree > 0).
		var cycleNames []string
		for _, n := range nodes {
			if inDegree[n] > 0 {
				cycleNames = append(cycleNames, n)
			}
		}
		return nil, fmt.Errorf("cycle: circular dependency among declarations: %v", cycleNames)
	}

	return order, nil
}

// collectIdents walks an expression tree and returns all IdentExpr.Name values
// found in leaf positions. Used to extract dependency edges for the toposort.
func collectIdents(expr ast.Expr) []string {
	if expr == nil {
		return nil
	}

	var result []string

	switch e := expr.(type) {
	case *ast.IdentExpr:
		result = append(result, e.Name)

	case *ast.BinaryExpr:
		result = append(result, collectIdents(e.Left)...)
		result = append(result, collectIdents(e.Right)...)

	case *ast.UnaryExpr:
		result = append(result, collectIdents(e.Operand)...)

	case *ast.IfThenElse:
		result = append(result, collectIdents(e.Cond)...)
		result = append(result, collectIdents(e.Then)...)
		result = append(result, collectIdents(e.Else)...)

	case *ast.IndexExpr:
		result = append(result, collectIdents(e.Collection)...)
		result = append(result, collectIdents(e.Index)...)

	case *ast.GetExpr:
		result = append(result, collectIdents(e.Object)...)

	case *ast.ApplyExpr:
		for _, arg := range e.Args {
			result = append(result, collectIdents(arg)...)
		}

	case *ast.ArrayExpr:
		for _, item := range e.Items {
			result = append(result, collectIdents(item)...)
		}

	case *ast.MapExpr:
		for _, item := range e.Items {
			result = append(result, collectIdents(item.Key)...)
			result = append(result, collectIdents(item.Value)...)
		}

	case *ast.PairExpr:
		result = append(result, collectIdents(e.Left)...)
		result = append(result, collectIdents(e.Right)...)

	case *ast.StructExpr:
		for _, m := range e.Members {
			result = append(result, collectIdents(m.Value)...)
		}

	case *ast.StringExpr:
		for _, part := range e.Parts {
			if ph, ok := part.(*ast.PlaceholderPart); ok {
				result = append(result, collectIdents(ph.Expr)...)
			}
		}

	// Literals have no ident children.
	case *ast.BoolLit, *ast.IntLit, *ast.FloatLit, *ast.NullLit:
		// no-op

	case *ast.CommandExpr:
		for _, part := range e.Parts {
			if ph, ok := part.(*ast.PlaceholderPart); ok {
				result = append(result, collectIdents(ph.Expr)...)
			}
		}
	}

	return result
}
