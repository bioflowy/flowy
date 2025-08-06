package errors

// SourcePosition represents a position in WDL source code
type SourcePosition struct {
	URI       string // filename/URI passed to WDL.load or import statement
	AbsPath   string // absolute filename/URI
	Line      int    // one-based line number
	Column    int    // one-based column number
	EndLine   int    // one-based end line number
	EndColumn int    // one-based end column number
}

// SourceNode represents the interface for AST nodes with source position
type SourceNode interface {
	GetPos() SourcePosition
	SetParent(parent SourceNode)
	GetParent() SourceNode
	Children() []SourceNode
	Less(other SourceNode) bool
	Equal(other SourceNode) bool
}

// BaseSourceNode provides a basic implementation of SourceNode
type BaseSourceNode struct {
	Pos    SourcePosition
	Parent SourceNode
}

func (n *BaseSourceNode) GetPos() SourcePosition {
	return n.Pos
}

func (n *BaseSourceNode) SetParent(parent SourceNode) {
	n.Parent = parent
}

func (n *BaseSourceNode) GetParent() SourceNode {
	return n.Parent
}

func (n *BaseSourceNode) Children() []SourceNode {
	return []SourceNode{}
}

func (n *BaseSourceNode) Less(other SourceNode) bool {
	if other == nil {
		return false
	}
	otherPos := other.GetPos()
	
	if n.Pos.AbsPath != otherPos.AbsPath {
		return n.Pos.AbsPath < otherPos.AbsPath
	}
	if n.Pos.Line != otherPos.Line {
		return n.Pos.Line < otherPos.Line
	}
	if n.Pos.Column != otherPos.Column {
		return n.Pos.Column < otherPos.Column
	}
	if n.Pos.EndLine != otherPos.EndLine {
		return n.Pos.EndLine < otherPos.EndLine
	}
	return n.Pos.EndColumn < otherPos.EndColumn
}

func (n *BaseSourceNode) Equal(other SourceNode) bool {
	if other == nil {
		return false
	}
	return n.Pos == other.GetPos()
}