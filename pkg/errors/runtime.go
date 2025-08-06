package errors

import (
	"fmt"
	"sort"
)

// MultipleValidationErrors propagates several validation/typechecking errors
type MultipleValidationErrors struct {
	Exceptions         []error
	SourceText         *string
	DeclaredWDLVersion *string
}

func NewMultipleValidationErrors(exceptions ...error) *MultipleValidationErrors {
	var allExceptions []error

	for _, exc := range exceptions {
		if multiExc, ok := exc.(*MultipleValidationErrors); ok {
			allExceptions = append(allExceptions, multiExc.Exceptions...)
		} else {
			allExceptions = append(allExceptions, exc)
		}
	}

	// Sort by position if possible
	sort.Slice(allExceptions, func(i, j int) bool {
		// Try to get position from errors that have it
		var posI, posJ *SourcePosition

		if errI, ok := allExceptions[i].(*ValidationError); ok {
			posI = &errI.Pos
		}
		if errJ, ok := allExceptions[j].(*ValidationError); ok {
			posJ = &errJ.Pos
		}

		if posI != nil && posJ != nil {
			if posI.AbsPath != posJ.AbsPath {
				return posI.AbsPath < posJ.AbsPath
			}
			if posI.Line != posJ.Line {
				return posI.Line < posJ.Line
			}
			return posI.Column < posJ.Column
		}

		return false // Keep original order if positions not available
	})

	return &MultipleValidationErrors{
		Exceptions: allExceptions,
	}
}

func (e *MultipleValidationErrors) Error() string {
	if len(e.Exceptions) == 1 {
		return e.Exceptions[0].Error()
	}
	return fmt.Sprintf("Multiple validation errors (%d)", len(e.Exceptions))
}

// RuntimeError represents backend-specific runtime errors
type RuntimeError struct {
	Message  string
	MoreInfo map[string]any
}

func NewRuntimeError(message string, moreInfo map[string]any) *RuntimeError {
	if moreInfo == nil {
		moreInfo = make(map[string]any)
	}
	return &RuntimeError{
		Message:  message,
		MoreInfo: moreInfo,
	}
}

func (e *RuntimeError) Error() string {
	return e.Message
}

// EvalError represents error evaluating a WDL expression or declaration
type EvalError struct {
	*RuntimeError
	Pos  SourcePosition
	Node SourceNode
}

func NewEvalError(node SourceNode, message string) *EvalError {
	return &EvalError{
		RuntimeError: NewRuntimeError(message, nil),
		Pos:          node.GetPos(),
		Node:         node,
	}
}

func NewEvalErrorFromPos(pos SourcePosition, message string) *EvalError {
	return &EvalError{
		RuntimeError: NewRuntimeError(message, nil),
		Pos:          pos,
	}
}

// OutOfBounds represents out of bounds error
type OutOfBounds struct {
	*EvalError
}

func NewOutOfBounds(node SourceNode, message string) *OutOfBounds {
	return &OutOfBounds{NewEvalError(node, message)}
}

// EmptyArray represents empty array error
type EmptyArray struct {
	*EvalError
}

func NewEmptyArray(node SourceNode) *EmptyArray {
	return &EmptyArray{NewEvalError(node, "Empty array for Array+ input/declaration")}
}

// NullValue represents null value error
type NullValue struct {
	*EvalError
}

func NewNullValue(node SourceNode) *NullValue {
	return &NullValue{NewEvalError(node, "Null value")}
}

func NewNullValueFromPos(pos SourcePosition) *NullValue {
	return &NullValue{NewEvalErrorFromPos(pos, "Null value")}
}

// InputError represents error reading an input value/file
type InputError struct {
	*RuntimeError
}

func NewInputError(message string, moreInfo map[string]any) *InputError {
	return &InputError{NewRuntimeError(message, moreInfo)}
}
