# miniwdl Go移植 TodoList

## 完了済みフェーズ ✅

### Phase 1: 基盤パッケージ (pkg/errors) ✅
- [x] Error.pyのテストを作成  
- [x] Error.pyをGoに移植（472行）
- [x] Errorパッケージのテストを実行・検証（222行テスト）

### Phase 2: Utils パッケージ (pkg/utils) ✅
- [x] _util.pyのテストを作成
- [x] _util.pyをGoに移植（325行）
- [x] utilパッケージのテストを実行・検証（352行テスト）

### Phase 3: Env パッケージ (pkg/env) ✅
- [x] Env.pyのテストを作成
- [x] Env.pyをGoに移植（243行）
- [x] envパッケージのテストを実行・検証（728行テスト）

### Phase 4: Type パッケージ (pkg/types) ✅
- [x] Type.pyのテストを作成
- [x] Type.pyをGoに移植（1,592行）
- [x] typesパッケージのテストを実行・検証（380行テスト）

### Phase 5: Value パッケージ (pkg/values) ✅
- [x] Value.pyのテストを作成
- [x] Value.pyをGoに移植（2,075行）
- [x] valuesパッケージのテストを実行・検証（1,064行テスト）

### Phase 6: Expr パッケージ (pkg/expr) ✅
- [x] Expr.pyのテストを作成
- [x] Expr.pyをGoに移植（3,028行）
- [x] exprパッケージのテストを実行・検証（1,102行テスト）

### Phase 7: Tree パッケージ (pkg/tree) ✅
- [x] Tree.pyのテストを作成
- [x] Tree.pyをGoに移植（1,289行）
- [x] treeパッケージのテストを実行・検証（315行テスト）

### Phase 8: Parser パッケージ (pkg/parser) ✅
- [x] 手書きパーサーの実装（3,847行）
- [x] 53のWDL文法規則を完全実装
- [x] parserパッケージのテストを実行・検証（1,425行テスト）

## 未実装フェーズ（残り作業）

### Phase 9: StdLib パッケージ (pkg/stdlib)
- [ ] StdLib.pyのテストを作成
- [ ] 基本演算子の実装（_add, _sub, _mul, _div, _rem等）
- [ ] 論理演算子の実装（_land, _lor, _negate等）
- [ ] 比較演算子の実装（_eqeq, _neq, _lt, _lte, _gt, _gte）
- [ ] 数学関数の実装（floor, ceil, round）
- [ ] 文字列関数の実装（sub, basename, sep）
- [ ] 配列関数の実装（length, range, transpose, zip, cross, flatten）
- [ ] ファイル操作関数の実装（write_lines, write_tsv, write_map, write_json）
- [ ] ファイル読み込み関数の実装（read_lines, read_tsv, read_map, read_json, read_string, read_int, read_float, read_boolean）
- [ ] その他の関数（defined, select_first, select_all）
- [ ] stdlibパッケージのテストを実行・検証

### Phase 10: Lint パッケージ (pkg/lint)
- [ ] Lint.pyのテストを作成
- [ ] Linterベースクラスの実装
- [ ] 構文チェックルールの実装
- [ ] 命名規則チェックの実装
- [ ] 未使用変数・インポートの検出
- [ ] 型不整合の警告
- [ ] コードスタイルチェック
- [ ] lintパッケージのテストを実行・検証

### Phase 11: Walker パッケージ (pkg/walker)
- [ ] Walker.pyのテストを作成
- [ ] Walkerベースクラスの実装
- [ ] PreorderとPostorderトラバーサル
- [ ] SetParentsウォーカーの実装
- [ ] MarkImportsウォーカーの実装
- [ ] walkerパッケージのテストを実行・検証

### Phase 12: Runtime パッケージ - Core (pkg/runtime)
- [ ] runtime/__init__.pyのテストを作成
- [ ] 基本実行インターフェースの実装
- [ ] エラーハンドリング（error.py）の移植
- [ ] 設定管理（config.py）の移植
- [ ] キャッシュシステム（cache.py）の移植
- [ ] ダウンロード管理（download.py）の移植
- [ ] runtimeコアのテストを実行・検証

### Phase 13: Runtime パッケージ - Task実行 (pkg/runtime/task)
- [ ] task.pyのテストを作成
- [ ] ローカルタスク実行エンジンの実装
- [ ] タスクコンテナ管理（task_container.py）の移植
- [ ] 入出力ハンドリングの実装
- [ ] リソース管理の実装
- [ ] タスク実行のテストを実行・検証

### Phase 14: Runtime パッケージ - Workflow実行 (pkg/runtime/workflow)
- [ ] workflow.pyのテストを作成
- [ ] ワークフロー実行エンジンの実装
- [ ] Scatter/Gather処理の実装
- [ ] Conditional実行の実装
- [ ] 依存関係グラフの管理
- [ ] 並列実行制御の実装
- [ ] ワークフロー実行のテストを実行・検証

### Phase 15: Runtime パッケージ - バックエンド (pkg/runtime/backend)
- [ ] Dockerバックエンド（docker_swarm.py）の実装
- [ ] Singularityバックエンド（singularity.py）の実装
- [ ] Podmanバックエンド（podman.py）の実装
- [ ] CLIサブプロセスバックエンド（cli_subprocess.py）の実装
- [ ] バックエンド抽象化インターフェースの実装
- [ ] バックエンドのテストを実行・検証

### Phase 16: Zip パッケージ (pkg/zip)
- [ ] Zip.pyのテストを作成
- [ ] WDLソースアーカイブ機能の実装
- [ ] インポート解決とリライトの実装
- [ ] マニフェスト生成の実装
- [ ] 追加ファイルバンドリングの実装
- [ ] zipパッケージのテストを実行・検証

### Phase 17: CLI パッケージ (pkg/cli)
- [ ] CLI.pyのテストを作成
- [ ] checkコマンドの実装（構文チェック）
- [ ] runコマンドの実装（タスク/ワークフロー実行）
- [ ] evalコマンドの実装（式評価）
- [ ] zipコマンドの実装（アーカイブ作成）
- [ ] localizeコマンドの実装（ファイルローカライズ）
- [ ] configureコマンドの実装（設定管理）
- [ ] input-templateコマンドの実装（入力テンプレート生成）
- [ ] CLIのテストを実行・検証

### Phase 18: 統合とリリース準備
- [ ] 全パッケージの統合テスト作成
- [ ] エンドツーエンドテストの実装
- [ ] パフォーマンステストとベンチマーク
- [ ] メモリ使用量の最適化
- [ ] 並行処理の最適化
- [ ] APIドキュメントの生成
- [ ] 使用例とチュートリアルの作成
- [ ] READMEの更新
- [ ] ライセンスファイルの追加
- [ ] CIパイプラインの設定

## 進捗サマリー
- **完了済み**: 8/18フェーズ (44%)
- **総実装行数**: 11,516行（テスト: 5,942行）
- **次の作業**: Phase 9 (StdLib パッケージ)