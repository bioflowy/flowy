# Flowy - miniwdl Go移植プロジェクト

## プロジェクト概要
miniwdl（WDL Runtime）のGoへの移植プロジェクトです。PythonからGoへの段階的な移植を行い、依存関係の少ないモジュールから順番に実装しています。

## 移植方針
1. **テストファーストアプローチ**: 移植前に必ずテストを作成
2. **段階的移植**: 依存関係の少ないファイルから順番に移植
3. **品質保証**: 移植後は必ずテストが通ることを確認

## プロジェクト構造
```
pkg/
├── errors/    # WDLエラー型とハンドリング（完了）
├── utils/     # ユーティリティ関数（完了）
├── env/       # 環境とバインディング（完了）
├── types/     # WDL型システム（完了）
├── values/    # WDL値システム（完了）
├── expr/      # WDL式システム（完了）
├── tree/      # WDL AST文書構造（完了）
└── parser/    # WDL手書きパーサー（完了）
```

## コーディング規約
- **ファイルサイズ制限**: 1つのGoファイルは最大500行、理想的には200行以内
- **パッケージ分割**: 大きなファイルは機能別に複数ファイルに分割
- **テストカバレッジ**: 全ての公開関数にテストを作成
- **エラーハンドリング**: Goの慣用的なエラーハンドリングを使用

## 完了済みパッケージ

### pkg/errors (472行 + 222行テスト)
WDLのエラー型とエラーハンドリングを実装。コーディング規約に従い4ファイルに機能分割。

**ファイル構成:**
- `source.go` (70行): SourcePosition、SourceNodeの定義
- `syntax.go` (57行): SyntaxError、ImportError等のパースエラー
- `validation.go` (203行): ValidationError系の静的検証エラー
- `runtime.go` (142行): RuntimeError、EvalError系の実行時エラー

**主要な型:**
- `SourcePosition`: ソース位置情報
- `SourceNode`: ASTノードのインターフェース
- `ValidationError`: 静的検証エラーの基底型
- `RuntimeError`: 実行時エラーの基底型
- 33種類の具体的なエラー型

### pkg/utils (325行 + 352行テスト)  
WDL処理のためのユーティリティ関数を実装。

**主要な機能:**
- `StripLeadingWhitespace`: 共通の先頭空白除去
- `AdjM`: 位相ソート用の隣接行列
- `TopSort`: 位相ソートアルゴリズム
- `WriteAtomic`: アトミックファイル書き込み
- `SymlinkForce`: 強制シンボリックリンク作成
- `ParseByteSize`: バイトサイズ文字列パーサー
- `PathReallyWithin`: 安全なパス検証

### pkg/env (243行 + 728行テスト)
環境とバインディングシステムの実装。

**ファイル構成:**
- `env.go` (243行): Binding、Bindingsの実装

**主要な型:**
- `Binding[T]`: 名前と値のバインディング
- `Bindings[T]`: バインディングのリンクリスト
- 名前空間サポート、マージ機能

### pkg/types (1,592行 + 380行テスト)
WDL型システムの実装。

**ファイル構成:**
- `base.go` (124行): Base interfaceとAnyType
- `primitive.go` (362行): Boolean、Int、Float、String、File、Directory型
- `composite.go` (441行): Array、Map、Pair、Struct、Object型
- `utilities.go` (151行): 型統一とヘルパー関数

**主要な機能:**
- 基本型: `Int`, `Float`, `String`, `Boolean`, `File`, `Directory`
- 複合型: `Array`, `Map`, `Pair`, `Struct`, `Object`
- 型変換と型チェック機能
- 型統一（Unify）アルゴリズム

### pkg/values (2,075行 + 1,064行テスト)
WDL値システムの実装。

**ファイル構成:**
- `base.go` (62行): Base interfaceとNull値
- `primitive.go` (349行): 基本型の値クラス
- `composite.go` (703行): 複合型の値クラス
- `utilities.go` (347行): JSONパース、パス書き換え
- `json.go` (29行): JSON変換ユーティリティ

**主要な機能:**
- 各型に対応する値クラス（BooleanValue、IntValue等）
- JSON変換機能（FromJSON、ToJSON）
- パス書き換え機能（RewritePaths）
- 値の強制変換（Coerce）と等価性チェック

### pkg/expr (3,028行 + 1,102行テスト)
WDL式システムの実装。

**ファイル構成:**
- `base.go` (54行): 基底型とStdLib interface
- `literals.go` (267行): リテラル式（Boolean、Int、Float等）
- `references.go` (150行): 識別子、属性アクセス、配列アクセス
- `functions.go` (183行): 関数呼び出し、二項演算、単項演算
- `collections.go` (388行): 配列、マップ、ペア、構造体リテラル
- `control.go` (89行): if-then-else制御構造
- `strings.go` (163行): 文字列補間とプレースホルダー

**主要な機能:**
- 式ASTノード（Literal、Apply、BinaryOp等）
- 式評価エンジン（Eval）
- 型推論システム（InferType）
- 型チェック（TypeCheck）

### pkg/tree (1,289行 + 315行テスト)  
WDL AST文書構造の実装。

**ファイル構成:**
- `base.go` (202行): 基底型とノードインターフェース
- `declaration.go` (301行): 宣言、入力、出力セクション
- `workflow.go` (338行): ワークフロー、呼び出し、制御構造
- `task.go` (185行): タスク定義
- `document.go` (148行): WDLドキュメント全体
- `helpers.go` (115行): グラフ構築ヘルパー

**主要な機能:**
- ワークフローとタスクのAST表現
- 宣言、入力、出力セクション
- 制御構造（scatter、if、call）

### pkg/parser (3,847行 + 1,425行テスト)
WDL手書きパーサーの実装。53の文法規則を網羅。

**ファイル構成:**
- `lexer.go` (741行): WDLトークナイザー（108種のトークン）
- `parser.go` (268行): パーサー基盤とエラーハンドリング
- `util.go` (186行): パースユーティリティ
- `types.go` (344行): 型パーサー（Array、Map、Pair等）
- `expressions.go` (533行): 式パーサー（演算子、関数呼び出し等）
- `declarations.go` (342行): 宣言パーサー（input、output等）
- `workflows.go` (387行): ワークフローパーサー（scatter、if、call）
- `tasks.go` (420行): タスクパーサー（command、meta等）
- `documents.go` (238行): ドキュメントパーサー（version、import）
- `strings.go` (337行): 文字列と補間パーサー
- `literals.go` (92行): リテラルパーサー

**主要な機能:**
- 再帰下降パーサーによるWDL構文解析
- エラー回復機能
- 53の文法規則を完全実装
- 統合テストによる品質保証

## 次期実装予定

### Phase 9: pkg/runtime
WDLランタイム・実行エンジンの実装。
- ワークフロー実行エンジン
- タスク実行管理
- 並列処理とスケジューリング
- リソース管理

### Phase 10: pkg/stdlib
WDL標準ライブラリの拡張実装。
- ファイル操作関数
- 文字列処理関数
- 数学・統計関数
- バイオインフォマティクス特化関数

### Phase 11: 統合システム
全パッケージの統合とCLIツール。
- CLIインターフェース
- 設定ファイル管理
- ログとモニタリング
- パフォーマンス最適化

## 開発履歴
- 2025-08-06: プロジェクト開始、errors・utilsパッケージ完了（1,370行）
- 2025-08-06: pkg/env完了（971行）、pkg/types完了（1,972行）、pkg/values完了（3,139行）
- 2025-08-07: pkg/expr完了（4,130行）、pkg/tree完了（1,604行）、pkg/parser完了（5,272行）

## 統計情報
- **総行数**: 17,458行（実装：11,516行 + テスト：5,942行）
- **完了済みパッケージ**: 8/8パッケージ（100%完了）
- **テストカバレッジ**: 全パッケージでテスト実装済み
- **文法規則**: 53のWDL文法規則を完全実装

## テスト実行
```bash
# 全テスト実行
go test ./...

# 個別パッケージテスト
go test ./pkg/errors
go test ./pkg/utils  
go test ./pkg/env
go test ./pkg/types
go test ./pkg/values
```

## 依存関係
- `github.com/google/uuid`: UUID生成

## 技術的な注意点
- Pythonの多重継承をGoのembeddingで実現
- ジェネリクスを活用した型安全性の確保
- interfaceを使ったポリモーフィズムの実現
- テストでのtempDirを使った安全なファイル操作