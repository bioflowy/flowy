package ast

// TokenType is an alias for int that matches lexer.TokenType.
// Using an alias (= int) rather than a defined type avoids a circular import
// between wdl/ast and wdl/lexer, since the lexer will import wdl/ast.
type TokenType = int // matches lexer.TokenType

// Expr is the marker interface implemented by all WDL expression node types.
type Expr interface {
	exprNode()
}

// BoolLit is a boolean literal: true or false.
type BoolLit struct {
	Pos
	Value bool
}

func (*BoolLit) exprNode() {}

// IntLit is an integer literal.
type IntLit struct {
	Pos
	Value int64
}

func (*IntLit) exprNode() {}

// FloatLit is a floating-point literal.
type FloatLit struct {
	Pos
	Value float64
}

func (*FloatLit) exprNode() {}

// NullLit is the WDL null literal.
type NullLit struct {
	Pos
}

func (*NullLit) exprNode() {}

// StringExpr is a string expression, potentially containing interpolation placeholders.
// Parts is a sequence of RawStringPart and PlaceholderPart segments.
type StringExpr struct {
	Pos
	Parts []StringPart
}

func (*StringExpr) exprNode() {}

// CommandExpr is a command block expression used in a WDL task.
// Style indicates whether it is a brace ({ }) or heredoc (<<< >>>) command block.
type CommandExpr struct {
	Pos
	Parts []StringPart
	Style CommandStyle
}

func (*CommandExpr) exprNode() {}

// ArrayExpr is an array literal expression: [item1, item2, ...].
type ArrayExpr struct {
	Pos
	Items []Expr
}

func (*ArrayExpr) exprNode() {}

// MapItem is a single key-value pair within a MapExpr.
type MapItem struct {
	Key   Expr
	Value Expr
}

// MapExpr is a map literal expression: {"key": value, ...}.
type MapExpr struct {
	Pos
	Items []MapItem
}

func (*MapExpr) exprNode() {}

// PairExpr is a pair literal expression: (left, right).
type PairExpr struct {
	Pos
	Left  Expr
	Right Expr
}

func (*PairExpr) exprNode() {}

// StructMember is a single field assignment within a StructExpr.
type StructMember struct {
	Name  string
	Value Expr
}

// StructExpr is a struct literal expression: MyStruct { field: value, ... }.
type StructExpr struct {
	Pos
	TypeName string
	Members  []StructMember
}

func (*StructExpr) exprNode() {}

// IfThenElse is a WDL conditional expression: if Cond then Then else Else.
type IfThenElse struct {
	Pos
	Cond Expr
	Then Expr
	Else Expr
}

func (*IfThenElse) exprNode() {}

// BinaryExpr is a binary operator expression (e.g., a + b, x && y).
// Op is a TokenType (int alias) matching the operator token from the lexer.
type BinaryExpr struct {
	Pos
	Op    TokenType
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

// UnaryExpr is a unary operator expression (e.g., -x, !b).
// Op is a TokenType (int alias) matching the operator token from the lexer.
type UnaryExpr struct {
	Pos
	Op      TokenType
	Operand Expr
}

func (*UnaryExpr) exprNode() {}

// IndexExpr is an index access expression: collection[index].
type IndexExpr struct {
	Pos
	Collection Expr
	Index      Expr
}

func (*IndexExpr) exprNode() {}

// GetExpr is a member access expression: object.member.
type GetExpr struct {
	Pos
	Object Expr
	Member string
}

func (*GetExpr) exprNode() {}

// ApplyExpr is a function call expression: name(arg1, arg2, ...).
type ApplyExpr struct {
	Pos
	Name string
	Args []Expr
}

func (*ApplyExpr) exprNode() {}

// IdentExpr is an identifier reference expression.
type IdentExpr struct {
	Pos
	Name string
}

func (*IdentExpr) exprNode() {}
