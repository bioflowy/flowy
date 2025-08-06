// Package utils provides utility functions for WDL processing
package utils

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// StripLeadingWhitespace removes common leading whitespace from all lines
func StripLeadingWhitespace(txt string) (int, string) {
	lines := strings.Split(txt, "\n")

	var toStrip *int
	for _, line := range lines {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if len(trimmed) > 0 {
			leadingSpaces := len(line) - len(trimmed)
			if toStrip == nil || *toStrip > leadingSpaces {
				toStrip = &leadingSpaces
			}
		}
	}

	if toStrip == nil || *toStrip == 0 {
		return 0, txt
	}

	for i, line := range lines {
		if len(strings.TrimSpace(line)) > 0 && len(line) >= *toStrip {
			lines[i] = line[*toStrip:]
		}
	}

	return *toStrip, strings.Join(lines, "\n")
}

// AdjM represents a sparse adjacency matrix for topological sorting
type AdjM[T comparable] struct {
	forward       map[T]map[T]bool
	reverse       map[T]map[T]bool
	unconstrained map[T]bool
}

// NewAdjM creates a new adjacency matrix
func NewAdjM[T comparable]() *AdjM[T] {
	return &AdjM[T]{
		forward:       make(map[T]map[T]bool),
		reverse:       make(map[T]map[T]bool),
		unconstrained: make(map[T]bool),
	}
}

// GetSinks returns all nodes that this source points to
func (a *AdjM[T]) GetSinks(source T) []T {
	var sinks []T
	for sink := range a.forward[source] {
		sinks = append(sinks, sink)
	}
	return sinks
}

// GetSources returns all nodes that point to this sink
func (a *AdjM[T]) GetSources(sink T) []T {
	var sources []T
	for source := range a.reverse[sink] {
		sources = append(sources, source)
	}
	return sources
}

// GetNodes returns all nodes in the graph
func (a *AdjM[T]) GetNodes() []T {
	var nodes []T
	for node := range a.forward {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetUnconstrained returns all unconstrained nodes
func (a *AdjM[T]) GetUnconstrained() []T {
	var nodes []T
	for node := range a.unconstrained {
		nodes = append(nodes, node)
	}
	return nodes
}

// AddNode adds a node to the graph
func (a *AdjM[T]) AddNode(node T) {
	if _, exists := a.forward[node]; !exists {
		a.forward[node] = make(map[T]bool)
		a.reverse[node] = make(map[T]bool)
		a.unconstrained[node] = true
	}
}

// AddEdge adds an edge from source to sink
func (a *AdjM[T]) AddEdge(source, sink T) {
	a.AddNode(source)
	a.AddNode(sink)

	if !a.forward[source][sink] {
		a.forward[source][sink] = true
		a.reverse[sink][source] = true
		delete(a.unconstrained, sink)
	}
}

// RemoveEdge removes an edge from source to sink
func (a *AdjM[T]) RemoveEdge(source, sink T) {
	if a.forward[source] != nil && a.forward[source][sink] {
		delete(a.forward[source], sink)
		delete(a.reverse[sink], source)
		if len(a.reverse[sink]) == 0 {
			a.unconstrained[sink] = true
		}
	}
}

// RemoveNode removes a node and all its edges
func (a *AdjM[T]) RemoveNode(node T) {
	// Remove edges pointing to this node
	for source := range a.reverse[node] {
		a.RemoveEdge(source, node)
	}
	// Remove edges pointing from this node
	for sink := range a.forward[node] {
		a.RemoveEdge(node, sink)
	}

	delete(a.forward, node)
	delete(a.reverse, node)
	delete(a.unconstrained, node)
}

// TopSort performs topological sorting on the adjacency matrix
func TopSort[T comparable](adj *AdjM[T]) []T {
	var result []T

	for len(adj.unconstrained) > 0 {
		// Get any unconstrained node
		var node T
		for n := range adj.unconstrained {
			node = n
			break
		}

		adj.RemoveNode(node)
		result = append(result, node)
	}

	// Check for cycles
	if len(adj.forward) > 0 {
		panic("cycle detected in graph")
	}

	return result
}

// WriteAtomic writes content to a file atomically
func WriteAtomic(contents, filename string) error {
	return WriteAtomicWithEnd(contents, filename, "\n")
}

// WriteAtomicWithEnd writes content to a file atomically with custom ending
func WriteAtomicWithEnd(contents, filename, end string) error {
	tempFile := filename + ".tmp." + uuid.New().String()

	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	_, err = f.WriteString(contents + end)
	if err != nil {
		f.Close()
		os.Remove(tempFile)
		return err
	}

	err = f.Close()
	if err != nil {
		os.Remove(tempFile)
		return err
	}

	return os.Rename(tempFile, filename)
}

// SymlinkForce creates a symbolic or hard link, replacing any existing link
func SymlinkForce(src, dst string, hard bool) error {
	tempLink := dst + ".tmp." + uuid.New().String()

	var err error
	if hard {
		err = os.Link(src, tempLink)
	} else {
		err = os.Symlink(src, tempLink)
	}

	if err != nil {
		return err
	}

	return os.Rename(tempLink, dst)
}

// ParseByteSize parses a byte size string (e.g., "1K", "1MB", "1.5G")
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte size string")
	}

	// Check for negative values
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("negative byte size not allowed")
	}

	// Find the first non-digit, non-decimal point character
	var numPart, unitPart string
	for i, r := range s {
		if !unicode.IsDigit(r) && r != '.' {
			numPart = s[:i]
			unitPart = strings.ToUpper(s[i:])
			break
		}
	}

	if numPart == "" {
		numPart = s
		unitPart = ""
	}

	num, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", numPart)
	}

	if num < 0 {
		return 0, fmt.Errorf("negative byte size not allowed")
	}

	var multiplier int64 = 1
	switch unitPart {
	case "", "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unitPart)
	}

	result := int64(math.Round(num * float64(multiplier)))
	return result, nil
}

// SplitAll splits a path into all its components
func SplitAll(path string) []string {
	if path == "" {
		return []string{}
	}

	var parts []string
	path = filepath.Clean(path)

	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}

		if dir == "" || dir == "/" {
			if dir == "/" {
				parts = append([]string{"/"}, parts...)
			}
			break
		}

		path = filepath.Clean(dir)
	}

	return parts
}

// PathReallyWithin checks if child path is really within parent path
func PathReallyWithin(child, parent string) bool {
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}

	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}

	// Ensure paths end with separator for proper comparison
	if !strings.HasSuffix(parentAbs, string(filepath.Separator)) {
		parentAbs += string(filepath.Separator)
	}

	if !strings.HasSuffix(childAbs, string(filepath.Separator)) {
		childAbs += string(filepath.Separator)
	}

	// Handle the case where child equals parent
	if childAbs == parentAbs {
		return true
	}

	return strings.HasPrefix(childAbs, parentAbs)
}
