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
├── env/       # 環境とバインディング（次期実装予定）
├── types/     # WDL型システム（次期実装予定）
├── values/    # WDL値システム（次期実装予定）
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

## 次期実装予定

### Phase 3: pkg/env
環境とバインディングシステムの実装。
- `Binding[T]`: 名前と値のバインディング
- `Bindings[T]`: バインディングのリンクリスト

### Phase 4: pkg/types
WDL型システムの実装。
- 基本型: `Int`, `Float`, `String`, `Boolean`, `File`
- 複合型: `Array`, `Map`, `Pair`, `Struct`
- 型変換と型チェック機能

### Phase 5: pkg/values
WDL値システムの実装。
- 各型に対応する値クラス
- JSON変換機能
- パス書き換え機能

### Phase 6: pkg/expr
WDL式システムの実装。
- 式ASTノード
- 式評価エンジン
- 型推論システム

## 開発履歴
- 2025-08-06: プロジェクト開始、errors・utilsパッケージ完了（1,370行）

## テスト実行
```bash
# 全テスト実行
go test ./...

# 個別パッケージテスト
go test ./pkg/errors
go test ./pkg/utils
```

## 依存関係
- `github.com/google/uuid`: UUID生成

## 技術的な注意点
- Pythonの多重継承をGoのembeddingで実現
- ジェネリクスを活用した型安全性の確保
- interfaceを使ったポリモーフィズムの実現
- テストでのtempDirを使った安全なファイル操作