package errors

import "fmt"

// ValidationError represents base class for WDL validation errors
type ValidationError struct {
	Pos                 SourcePosition
	Node                SourceNode
	Message             string
	SourceText          *string
	DeclaredWDLVersion  *string
}

func NewValidationError(node SourceNode, message string) *ValidationError {
	return &ValidationError{
		Pos:     node.GetPos(),
		Node:    node,
		Message: message,
	}
}

func NewValidationErrorFromPos(pos SourcePosition, message string) *ValidationError {
	return &ValidationError{
		Pos:     pos,
		Message: message,
	}
}

func (e *ValidationError) Error() string {
	return e.Message
}

// InvalidType represents invalid type error
type InvalidType struct {
	*ValidationError
}

func NewInvalidType(node SourceNode, message string) *InvalidType {
	return &InvalidType{NewValidationError(node, message)}
}

// IndeterminateType represents indeterminate type error
type IndeterminateType struct {
	*ValidationError
}

func NewIndeterminateType(node SourceNode, message string) *IndeterminateType {
	return &IndeterminateType{NewValidationError(node, message)}
}

// NoSuchTask represents missing task/workflow error
type NoSuchTask struct {
	*ValidationError
	Name string
}

func NewNoSuchTask(node SourceNode, name string) *NoSuchTask {
	return &NoSuchTask{
		ValidationError: NewValidationError(node, "No such task/workflow: "+name),
		Name:           name,
	}
}

// NoSuchCall represents missing call error  
type NoSuchCall struct {
	*ValidationError
	Name string
}

func NewNoSuchCall(node SourceNode, name string) *NoSuchCall {
	return &NoSuchCall{
		ValidationError: NewValidationError(node, "No such call in this workflow: "+name),
		Name:           name,
	}
}

// NoSuchFunction represents missing function error
type NoSuchFunction struct {
	*ValidationError  
	Name string
}

func NewNoSuchFunction(node SourceNode, name string) *NoSuchFunction {
	return &NoSuchFunction{
		ValidationError: NewValidationError(node, "No such function: "+name),
		Name:           name,
	}
}

// WrongArity represents wrong number of arguments error
type WrongArity struct {
	*ValidationError
	Expected int
}

func NewWrongArity(node SourceNode, functionName string, expected int) *WrongArity {
	message := fmt.Sprintf("%s expects %d argument(s)", functionName, expected)
	return &WrongArity{
		ValidationError: NewValidationError(node, message),
		Expected:       expected,
	}
}

// NotAnArray represents not an array error
type NotAnArray struct {
	*ValidationError
}

func NewNotAnArray(node SourceNode) *NotAnArray {
	return &NotAnArray{NewValidationError(node, "Not an array")}
}

// NoSuchMember represents missing member error
type NoSuchMember struct {
	*ValidationError
	Member string
}

func NewNoSuchMember(node SourceNode, member string) *NoSuchMember {
	return &NoSuchMember{
		ValidationError: NewValidationError(node, fmt.Sprintf("No such member '%s'", member)),
		Member:         member,
	}
}

// IncompatibleOperand represents incompatible operand error
type IncompatibleOperand struct {
	*ValidationError
}

func NewIncompatibleOperand(node SourceNode, message string) *IncompatibleOperand {
	return &IncompatibleOperand{NewValidationError(node, message)}
}

// UnknownIdentifier represents unknown identifier error
type UnknownIdentifier struct {
	*ValidationError
}

func NewUnknownIdentifier(node SourceNode, message string) *UnknownIdentifier {
	if message == "" {
		message = "Unknown identifier " + fmt.Sprintf("%v", node)
	}
	return &UnknownIdentifier{NewValidationError(node, message)}
}

// NoSuchInput represents missing input error
type NoSuchInput struct {
	*ValidationError
	Name string
}

func NewNoSuchInput(node SourceNode, name string) *NoSuchInput {
	return &NoSuchInput{
		ValidationError: NewValidationError(node, "No such input "+name),
		Name:           name,
	}
}

// UncallableWorkflow represents uncallable workflow error
type UncallableWorkflow struct {
	*ValidationError
	Name string
}

func NewUncallableWorkflow(node SourceNode, name string) *UncallableWorkflow {
	message := fmt.Sprintf(
		"Cannot call subworkflow %s because its own calls have missing required inputs, "+
			"and/or it lacks an output section", name)
	return &UncallableWorkflow{
		ValidationError: NewValidationError(node, message),
		Name:           name,
	}
}

// MultipleDefinitions represents multiple definitions error
type MultipleDefinitions struct {
	*ValidationError
}

func NewMultipleDefinitions(node SourceNode, message string) *MultipleDefinitions {
	return &MultipleDefinitions{NewValidationError(node, message)}
}

// StrayInputDeclaration represents stray input declaration error
type StrayInputDeclaration struct {
	*ValidationError
}

func NewStrayInputDeclaration(node SourceNode, message string) *StrayInputDeclaration {
	return &StrayInputDeclaration{NewValidationError(node, message)}
}

// CircularDependencies represents circular dependencies error
type CircularDependencies struct {
	*ValidationError
}

func NewCircularDependencies(node SourceNode) *CircularDependencies {
	message := "circular dependencies"
	// Note: In Python version, this tries to get name from node attributes
	// We'll keep it simple for now
	return &CircularDependencies{NewValidationError(node, message)}
}