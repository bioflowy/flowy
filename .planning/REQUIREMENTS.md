# Requirements: flowy

**Defined:** 2026-03-10
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v1 Requirements

### Parser

- [ ] **PARSE-01**: WDL 1.2 文書（version宣言、task、workflow、struct、import）をパースしてASTを生成できる
- [ ] **PARSE-02**: レキサーが `command {}` スタイルと `command <<<>>>` スタイルを別々のモードで処理できる
- [ ] **PARSE-03**: `command {}` ブロック内で `~{}` および `${}` プレースホルダを認識できる
- [ ] **PARSE-04**: `command <<<>>>` ブロック内で `~{}` のみをプレースホルダとして認識し、`${}` はbashリテラルとして通過させる
- [ ] **PARSE-05**: 文字列リテラル内での式補間（`~{}` / `${}`）をネスト含めて正しく処理できる
- [ ] **PARSE-06**: Pratt式パーサが `**`（指数）含む全演算子を正しい優先順位でパースできる
- [ ] **PARSE-07**: ローカルファイルの import 文を解決して複数ファイルにまたがるドキュメントをパースできる
- [ ] **PARSE-08**: パースエラーを行番号・列番号付きで報告できる
- [ ] **PARSE-09**: struct 定義をパースしてネスト struct を含む AST ノードを生成できる

### Type System

- [x] **TYPE-01**: Primitive 型（Boolean, Int, Float, String, File）を実装し、型タグ付きの Value 構造体を持つ
- [x] **TYPE-02**: Directory 型（WDL 1.2 新機能）を実装する
- [x] **TYPE-03**: Compound 型（Array[T], Map[K,V], Pair[L,R], Struct）を実装する（型パラメータ付き）
- [x] **TYPE-04**: Optional 型（T?）を実装し、None リテラルの型推論を正しく処理する
- [ ] **TYPE-05**: WDL 1.2 の型強制ルール 6 種（Int→Float, T→T?, String↔File, T→Array[T]等）を shallow かつ非再帰的に実装する
- [ ] **TYPE-06**: 型チェッカーが task および workflow のすべての宣言を前方参照を含めて解決できる
- [ ] **TYPE-07**: scatter ブロック出力を `Array[T]` に昇格し、if ブロック出力を `T?` に昇格する（冪等）
- [ ] **TYPE-08**: Struct 型チェッカーがフィールド型を検証し、構造体リテラルとアクセスを型付けできる

### Expression Evaluator

- [ ] **EXPR-01**: 算術・比較・論理演算子（`**` 指数含む）を評価できる
- [ ] **EXPR-02**: if-then-else 式を評価できる
- [ ] **EXPR-03**: メンバーアクセス（`struct.field`, `pair.left`, `pair.right`）を評価できる
- [ ] **EXPR-04**: 配列・マップのインデックスアクセスを評価できる
- [ ] **EXPR-05**: 関数呼び出しを評価できる（stdlib 関数の呼び出し）
- [ ] **EXPR-06**: コマンドテンプレートの展開（プレースホルダ置換 + dedent アルゴリズム）を実装する
- [ ] **EXPR-07**: command block dedent を「プレースホルダをセンチネル置換してから最小インデント計算」の正しいアルゴリズムで実装する

### Standard Library

- [ ] **STDLIB-01**: ファイル I/O 関数を実装する: `read_lines`, `read_tsv`, `read_map`, `read_json`, `read_boolean`, `read_int`, `read_float`, `read_string`, `write_lines`, `write_tsv`, `write_map`, `write_json`
- [ ] **STDLIB-02**: ファイルシステム関数を実装する: `glob`, `basename`, `dirname`, `size`, `stdout`, `stderr`
- [ ] **STDLIB-03**: 配列関数を実装する: `length`, `flatten`, `transpose`, `zip`, `cross`, `prefix`, `suffix`, `quote`, `squote`, `unzip`, `select_all`, `select_first`, `chunk` (1.2新機能)
- [ ] **STDLIB-04**: 文字列関数を実装する: `sub`, `find` (1.2新機能), `matches` (1.2新機能), `join_paths` (1.2新機能)
- [ ] **STDLIB-05**: 型変換・ユーティリティ関数を実装する: `defined`, `keys`, `values` (1.2新機能), `as_map`, `as_pairs`, `collect_by_key`, `contains` (1.2新機能), `contains_key` (1.2新機能)
- [ ] **STDLIB-06**: 数値関数を実装する: `ceil`, `floor`, `round`（half-up丸め、bankers roundingではない）, `min`, `max`

### Task Execution

- [ ] **TASK-01**: task の input セクションを解析し、JSON入力から値をバインドできる
- [ ] **TASK-02**: task の runtime/requirements セクション（container, cpu, memory）を解析して Docker 実行に反映できる
- [ ] **TASK-03**: コマンドテンプレートを展開してコンテナに渡す文字列を生成できる
- [ ] **TASK-04**: 入力ファイルをコンテナ内パスにマウントする双方向パスマップを構築できる
- [ ] **TASK-05**: Docker イメージの pull → コンテナ作成 → 実行 → stdout/stderr キャプチャ → 削除のライフサイクルを実装する
- [ ] **TASK-06**: task の output セクションを評価してホストパスに出力を収集できる
- [ ] **TASK-07**: `ContainerRunner` インターフェース経由で Docker を呼び出し、単体テストでモック可能にする
- [ ] **TASK-08**: タスク失敗時に exit code、stderr を含むエラー情報を返す

### Workflow Execution

- [ ] **WF-01**: ワークフローの呼び出しグラフを解析して実行順序を決定する DAG を構築できる
- [ ] **WF-02**: `call` ブロックを実行して出力を環境に束縛できる
- [ ] **WF-03**: `scatter` ブロックを展開してタスクを並列実行し、出力を `Array[T]` に収集できる（errgroup + SetLimit）
- [ ] **WF-04**: `if` ブロックを評価し、条件が偽の場合の出力を `None`（Optional）として扱える
- [ ] **WF-05**: `after` キーワードによる明示的な実行順序依存を尊重できる
- [ ] **WF-06**: 並列実行に `errgroup.WithContext` + `SetLimit` を使い、goroutine リークを防止する
- [ ] **WF-07**: ワークフロー完了後に `outputs.json` を出力ディレクトリに書き出す
- [ ] **WF-08**: いずれかのタスクが失敗した場合、実行中のタスクをキャンセルして `error.json` を書き出す

### CLI

- [ ] **CLI-01**: `flowy check <file.wdl>` でパース + 型チェックのみを実行し、エラーを行番号付きで出力する
- [ ] **CLI-02**: `flowy run <file.wdl>` でワークフローを実行し、`--input` フラグで JSON 入力ファイルを受け取る
- [ ] **CLI-03**: `--task <name>` フラグで WDL 内の特定タスクのみを単独実行できる
- [ ] **CLI-04**: `--path <dir>` フラグで import 検索パスを追加できる
- [ ] **CLI-05**: `--dir <dir>` フラグで実行ディレクトリを指定できる
- [ ] **CLI-06**: 成功時は exit code 0、失敗時は非ゼロで終了する
- [ ] **CLI-07**: `--verbose` フラグでタスク stdout/stderr をターミナルに表示できる

## v2 Requirements

### サーバ/クライアント分離

- **SRV-01**: REST API サーバとして起動し、外部からワークフロー実行を制御できる
- **SRV-02**: プロジェクト単位でユーザ・ワークフロー・入力ファイル・出力結果を管理できる
- **SRV-03**: SQLite でプロジェクト・実行履歴・状態を永続化できる
- **SRV-04**: CLI クライアントがサーバの REST API を介してワークフローを操作できる

### 追加機能

- **EXT-01**: HTTP(S) import の解決
- **EXT-02**: 実行キャッシュ（タスク出力の再利用）
- **EXT-03**: ワークフロー実行の途中再開
- **EXT-04**: Docker 以外のバックエンド（Podman, Singularity）

## Out of Scope

| Feature | Reason |
|---------|--------|
| Object 型 | WDL 1.2 で deprecated。Struct で代替可能 |
| WDL draft-2/1.0/1.1 のサポート | 別文法が必要、保守コスト大 |
| HTTP(S) import 解決 | 認証・キャッシュが複雑。v2 |
| 実行キャッシュ | 安定ハッシュが必要。正確性を先に確立 |
| sub-workflow 呼び出し | call/scatter/if を先に安定させる |
| Podman/Singularity バックエンド | 1 つの完成度の高いバックエンドを優先 |
| WDL lint 機能 | 実行エンジンが優先 |
| WDL zip 機能 | 実行エンジンが優先 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TYPE-01 | Phase 1: Type Data Structures | Complete |
| TYPE-02 | Phase 1: Type Data Structures | Complete |
| TYPE-03 | Phase 1: Type Data Structures | Complete |
| TYPE-04 | Phase 1: Type Data Structures | Complete |
| PARSE-01 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-02 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-03 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-04 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-05 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-06 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-07 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-08 | Phase 2: Lexer, Parser, and AST | Pending |
| PARSE-09 | Phase 2: Lexer, Parser, and AST | Pending |
| TYPE-05 | Phase 3: Type Checker | Pending |
| TYPE-06 | Phase 3: Type Checker | Pending |
| TYPE-07 | Phase 3: Type Checker | Pending |
| TYPE-08 | Phase 3: Type Checker | Pending |
| EXPR-01 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-02 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-03 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-04 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-05 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-06 | Phase 4: Expression Evaluator and Standard Library | Pending |
| EXPR-07 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-01 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-02 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-03 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-04 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-05 | Phase 4: Expression Evaluator and Standard Library | Pending |
| STDLIB-06 | Phase 4: Expression Evaluator and Standard Library | Pending |
| TASK-01 | Phase 5: Task Execution | Pending |
| TASK-02 | Phase 5: Task Execution | Pending |
| TASK-03 | Phase 5: Task Execution | Pending |
| TASK-04 | Phase 5: Task Execution | Pending |
| TASK-05 | Phase 5: Task Execution | Pending |
| TASK-06 | Phase 5: Task Execution | Pending |
| TASK-07 | Phase 5: Task Execution | Pending |
| TASK-08 | Phase 5: Task Execution | Pending |
| WF-01 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-02 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-03 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-04 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-05 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-06 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-07 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| WF-08 | Phase 6: Workflow Execution (DAG Scheduler) | Pending |
| CLI-01 | Phase 7: CLI and Integration | Pending |
| CLI-02 | Phase 7: CLI and Integration | Pending |
| CLI-03 | Phase 7: CLI and Integration | Pending |
| CLI-04 | Phase 7: CLI and Integration | Pending |
| CLI-05 | Phase 7: CLI and Integration | Pending |
| CLI-06 | Phase 7: CLI and Integration | Pending |
| CLI-07 | Phase 7: CLI and Integration | Pending |

**Coverage:**
- v1 requirements: 57 total
- Mapped to phases: 57
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-10*
*Last updated: 2026-03-10 after roadmap revision (7 phases)*
