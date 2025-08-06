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
└── expr/      # WDL式システム（次期実装予定）
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

## 次期実装予定

### Phase 6: pkg/expr
WDL式システムの実装。
- 式ASTノード
- 式評価エンジン
- 型推論システム

## 開発履歴
- 2025-08-06: プロジェクト開始、errors・utilsパッケージ完了（1,370行）
- 2025-08-06: pkg/env完了（971行）、pkg/types完了（1,972行）、pkg/values完了（3,139行）

## 統計情報
- **総行数**: 7,452行（実装：4,832行 + テスト：2,620行）
- **完了済みパッケージ**: 5/6パッケージ（83%完了）
- **テストカバレッジ**: 全パッケージでテスト実装済み

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