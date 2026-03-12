# flowy 用語集

## WDL 関連

| 用語 | 説明 |
|------|------|
| **WDL** | Workflow Description Language。バイオインフォマティクス分野で広く使われるワークフロー記述言語。flowy は WDL 1.2 仕様を実装している |
| **Document** | WDL ソースファイルをパースした結果のトップレベル AST ノード。`version 1.2` 宣言、タスク定義、ワークフロー定義、構造体定義を含む |
| **Task** | 単一の計算単位の定義。入力宣言（`input`）、コマンド（`command`）、出力宣言（`output`）、実行環境（`runtime`）で構成される。Docker コンテナ内で実行される |
| **Workflow** | 複数のタスク呼び出しとその依存関係を記述した有向非巡回グラフ（DAG）。`call`、`scatter`、`if` などの制御構文を含む |
| **Call** | ワークフロー内でタスクを呼び出す構文。入力バインディング（`input: name = expr`）でタスクの入力を指定する |
| **Scatter** | コレクション（Array）の各要素に対して並列にタスクを実行する制御構文。各反復は独立した shard として実行される |
| **Conditional (if)** | 条件に基づいてタスク呼び出しを実行するかどうかを制御する構文 |
| **Decl** | 型付き変数宣言。`String name = "hello"` のように書く |
| **StructDef** | ユーザ定義の複合型。`struct Sample { String name; File bam }` のように定義する |
| **Runtime セクション** | タスクの実行環境を指定するブロック。Docker イメージ、CPU、メモリ等を定義する |
| **Command セクション** | タスク内で実行されるシェルコマンドのテンプレート。WDL の式で変数展開される。`command { ... }` または `command <<< ... >>>` 構文がある |
| **型 (Type)** | WDL の型システム。プリミティブ型（`Int`, `String`, `Boolean`, `Float`, `File`, `Directory`）と複合型（`Array[T]`, `Map[K,V]`, `Pair[L,R]`, `Struct`）がある。`T?` でオプショナル型を表す |

## flowy アーキテクチャ関連

| 用語 | 説明 |
|------|------|
| **Client** | CLI ツール（`flowy` バイナリ）。ユーザがワークフローを送信し結果を取得するためのインタフェース |
| **Server** | REST API サーバ（`flowy-server` バイナリ）。ワークフローの受付、DAG スケジューリング、タスクキューの管理を行う |
| **Daemon** | ワーカープロセス（`flowy-daemon` バイナリ）。Server からタスクを取得し Docker コンテナで実行する |
| **Run** | ワークフローの 1 回の実行インスタンス。UUID で識別される。`PENDING` → `RUNNING` → `COMPLETED`/`FAILED` と状態遷移する |
| **RunState** | Run の状態。`PENDING`（待機中）、`RUNNING`（実行中）、`COMPLETED`（完了）、`FAILED`（失敗）、`ABORTED`（中止） |
| **TaskState** | Task の状態。`PENDING`（待機中）、`CLAIMED`（Daemon が取得）、`RUNNING`（実行中）、`COMPLETED`（完了）、`FAILED`（失敗）、`ABORTED`（中止） |
| **ShardIndex** | scatter で生成されるタスクの反復インデックス。非 scatter タスクでは 0 |
| **base_dir** | 全ての実行データを格納するルートディレクトリ。デフォルトは `./flowy-data`。Server・Daemon・Client が共有する |
| **ステージング** | Client がユーザのファイルを `base_dir` 配下にコピーする処理。Server や Daemon からアクセス可能にするため |
| **Heartbeat** | Daemon が定期的に Server に送信する生存通知。タイムアウトすると Daemon のタスクは ABORTED になる |
| **Stale Detection** | Server がハートビートのないタスクを検知し、ABORTED に遷移させる仕組み |

## Docker 実行関連

| 用語 | 説明 |
|------|------|
| **PathMap** | ホスト上のファイルパスとコンテナ内のパスの双方向マッピング。bind-mount の構成とパスの変換に使用される |
| **bind-mount** | Docker のファイルマウント機構。ホストのディレクトリをコンテナ内の指定パスに接続する |
| **/mnt/inputs/** | コンテナ内の入力ファイルマウントポイント。read-only |
| **/mnt/outputs/** | コンテナ内の出力ディレクトリ。read-write。ホスト上の `{execDir}/work/` に対応する |
| **execDir** | タスクの 1 回の実行に対応する一時ディレクトリ。コマンドスクリプト、標準出力、終了コード等を格納する |
| **RunRequest** | Docker コンテナの実行要求。イメージ名、コマンド、bind-mount リスト、環境変数、リソース制限を含む |

## WDL 処理パイプライン関連

| 用語 | 説明 |
|------|------|
| **Lexer** | WDL ソースコードをトークン列に分解する字句解析器 |
| **Parser** | トークン列から AST を構築する構文解析器。Pratt パーシング方式を採用 |
| **Checker** | AST に対して型チェックと意味検証を行うセマンティック解析器 |
| **Eval** | WDL の式を評価し、ランタイム値（`values.Value`）を返す式評価器 |
| **Value** | WDL の実行時の値。`BooleanValue`, `IntValue`, `StringValue`, `FileValue`, `ArrayValue`, `MapValue` 等 |
| **DAG** | 有向非巡回グラフ。ワークフロー内のタスク間の依存関係を表現する |
| **Scheduler** | DAG をトポロジカル順序で走査し、依存が解決されたタスクから順に実行するスケジューラ |
| **stdlib** | WDL 標準ライブラリ関数群。`read_lines()`, `write_lines()`, `size()`, `basename()` 等 |

## ツール関連

| 用語 | 説明 |
|------|------|
| **spectool** | WDL 1.2 仕様への準拠をテストする外部ツール（openwdl/spectool）。詳細は [spectool.md](spectool.md) を参照 |
| **flowy.yaml** | flowy の設定ファイル。`base_dir`、サーバ/デーモン/クライアントの接続先等を定義する |
| **flowy-data/** | 実行データのルートディレクトリ。SQLite DB、各 Run のファイルを格納する |
