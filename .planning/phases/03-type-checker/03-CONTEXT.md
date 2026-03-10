# Phase 3: Type Checker - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

WDL 1.2 ドキュメントの全型宣言を検証・解決する。前方参照解決、scatter/if 出力の型昇格、6種の型強制ルール（shallow）、struct フィールド検証を実装し、`flowy check` が型エラーを行番号付きで報告できるようにする。式の評価（値の計算）は Phase 4 のスコープ。

</domain>

<decisions>
## Implementation Decisions

### パッケージ構造・API
- 新規パッケージ `wdl/checker` を作成
- 公開 API: `checker.Check(doc *ast.Document) (TypeEnv, []error)`
- `TypeEnv` は `map[string]types.Type`（キーは宣言名文字列、スコープはネスト構造で管理）
- 複数の型エラーを一括収集して返す（`flowy check` で全エラーを一度に報告）
- AST は変更しない（不変を維持）

### 前方参照解決アルゴリズム
- **Two-pass 方式**:
  - Pass 1: 全宣言をスキャンしてシンボルテーブル（TypeEnv）を事前構築
  - Pass 2: 構築済み TypeEnv に対して型検証を実行
- **cycle detection**: toposort（Kahn's algorithm）で循環依存を検出してエラー報告
- scatter/if 内の未定義変数参照はサイクルではなく「スコープルール違反エラー」として扱う

### scatter/if 型昇格
- Pass 2 中にリアルタイムで昇格（scatter ボディ解析時に内部宣言を外部スコープへ登録する際に自動適用）
- `arrayize(T) → Array[T]`、`optionalize(T) → T?` のヘルパー関数を用意
- **冪等チェック必須**: `optionalize` 内で `IsOptional()==true` なら昇格しない（T?? は生成しない）
- scatter 内に if がネストした場合: T → T? (if) → Array[T?] (scatter) と順次適用

### 型検証スコープ（Phase 3 で行うこと）
- input/output セクションの宣言型検証
- input 型ミスマッチ検出（例: Int 宣言に String を渡す → Phase 3 でエラー）
- struct フィールド型の検証・struct リテラルとフィールドアクセスの型付け
- プリミティブ型リテラル（Int リテラル、String リテラル等）の型を返す（式の超幅 literal）
- WDL 1.2 型強制ルール 6 種を shallow（複合型パラメータへの再帰なし）で実装
- 複雑な式ノード（関数呼び出し、演算子式など）の型推論は Phase 4 へ委譲

### WDL 1.2 型強制ルール（6種、shallow 実装）
- `Int → Float`
- `T → T?`
- `String ↔ File`
- `T → Array[T]`
- その他 WDL 1.2 spec 規定の残り 2 種
- shallow: 複合型パラメータへの再帰適用はしない

### Claude's Discretion
- TypeEnv のネストスコープの具体的なデータ構造（linked list of maps vs. stack など）
- エラー型の具体的な struct 定義（既存 ParseError と統一するか新規 TypeError か）
- toposort の具体的な実装詳細

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `wdl/types` パッケージ: `Type` インターフェース、全プリミティブ型・複合型・Optional 型が実装済み。`WithOptional(bool)` で冪等チェックしやすい設計。
- `wdl/ast` パッケージ: 全 AST ノードに `Pos`（File, Line, Col）が埋め込み済み。型エラーに正確なソース位置を付与できる。
- `wdl/parser` パッケージ: `ParseError` 構造体（行番号・列番号付き）が既に定義済み。TypeError の設計参考にできる。

### Established Patterns
- エラーは行番号・ファイルパス付きで報告（`Pos` 埋め込みパターンは全 AST ノードで統一）
- パッケージ間は interface 経由（`Type` interface、`ContainerRunner` interface）— checker も同様に interface 境界で設計

### Integration Points
- `checker.Check()` の入力は `parser.Parse()` が返す `*ast.Document`
- `checker.Check()` の出力 `TypeEnv` は Phase 4 の `evaluator` パッケージが入力として受け取る
- Phase 7 の `flowy check` CLI コマンドが `checker.Check()` を直接呼び出す

</code_context>

<specifics>
## Specific Ideas

- miniwdl の方針を踏襲: Two-pass + toposort による cycle detection（miniwdl は `_detect_cycles()` で Kahn's algorithm を使用）
- miniwdl の `arrayize`/`optionalize` ヘルパー関数の設計をそのまま Go に移植する

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-type-checker*
*Context gathered: 2026-03-10*
