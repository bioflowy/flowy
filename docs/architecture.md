# flowy 内部アーキテクチャ

## 全体構成

flowy は WDL 1.2 ワークフロー実行エンジンであり、3つのプロセスから構成される。

```
┌──────────┐      HTTP       ┌──────────────┐     共有FS      ┌──────────────┐     Docker API
│  Client   │ ──────────────→│    Server     │←──────────────→│    Daemon     │──────────────→ コンテナ
│ (flowy)   │  POST/GET      │(flowy-server) │  inputs.json   │(flowy-daemon) │  bind-mount
└──────────┘                 └──────────────┘  outputs.json   └──────────────┘
```

| プロセス | バイナリ | 役割 |
|----------|----------|------|
| Client | `flowy` | WDL ファイルと入力を送信し、結果を取得する CLI |
| Server | `flowy-server` | ワークフロー DAG をスケジューリングし、タスクをキューに入れる |
| Daemon | `flowy-daemon` | タスクをキューから取得し Docker コンテナで実行する |

3つのプロセスは **共有ファイルシステム**（`flowy-data/`）を通じてファイルを受け渡す。

---

## ファイル・パスの受け渡しフロー

### 概要図

```
ユーザのファイル                 共有FS上のステージング領域              Dockerコンテナ内
/home/user/input.txt  ──copy──→  flowy-data/runs/{runID}/inputs/input.txt
                                          │
                                          │ bind-mount (read-only)
                                          ▼
                                  /mnt/inputs/input.txt

                                  /mnt/outputs/result.txt
                                          │
                                          │ bind-mount (read-write)
                                          ▼
                                 flowy-data/runs/{runID}/tasks/{task}_0/{tempdir}/work/result.txt
```

### Step 1: Client → Server（ファイルステージング）

**処理場所**: `cmd/flowy/main.go`

1. CLI は `flowy.yaml` から `base_dir`（デフォルト `./flowy-data`）を読み込み、**絶対パスに変換**する
2. WDL ファイルのパスを解決する
   - WDL ファイルが `base_dir` の外にある場合、`{baseDir}/runs/{runID}/inputs/` にコピーする
   - 同階層の `.wdl` インポートファイルも同様にコピーする
3. 入力 JSON ファイル内のファイルパスを走査する
   - 各文字列値がファイルパスかどうかをヒューリスティックに判定（絶対パス or パス区切り文字を含む）
   - 実在するファイルで `base_dir` 外のものを `{baseDir}/runs/{runID}/inputs/` にコピー
   - JSON 内のパスをコピー先の絶対パスに書き換える
4. `POST /workflows` で `SubmitRequest` を送信する

```go
// SubmitRequest の構造（internal/api/types.go）
type SubmitRequest struct {
    WDLPath     string            // ステージ済み WDL ファイルの絶対パス
    InputsJSON  map[string]any    // ステージ済みファイルパスを含む入力 JSON
    RunID       string            // クライアント生成の UUID
    SearchPaths []string          // インポート検索パス
}
```

**ポイント**: この時点で全てのファイルパスは共有 FS 上の絶対パスに変換されている。

### Step 2: Server（ワークフロースケジューリング）

**処理場所**: `internal/server/handlers.go`, `internal/server/dispatch.go`

1. `handleSubmit()` で WDL をパースし型チェックを行う
2. `store.CreateRunDirs()` で実行ディレクトリを作成する
3. バックグラウンドで `runWorkflow()` を起動し、DAG スケジューラがタスクを順序制御する
4. 各タスクについて `makeDispatchFn()` が以下を行う:
   - `{baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}/` ディレクトリを作成
   - タスク入力を `inputs.json` として書き出す
   - DB にタスクレコードを作成（状態: `PENDING`）
   - Daemon がタスクを完了するまでポーリングで待機

### Step 3: Server → Daemon（タスクの受け渡し）

**処理場所**: `internal/daemon/daemon.go`, `internal/server/handlers.go`

1. Daemon は `POST /tasks/claim` でタスクをポーリングする
2. Server は `TaskClaimResponse` を返す:

```go
type TaskClaimResponse struct {
    TaskID     string  // タスク DB ID
    RunID      string  // ワークフロー実行 ID
    TaskName   string  // タスク名
    ShardIndex int     // scatter インデックス（非scatter時は0）
    InputsDir  string  // 絶対パス: {baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}
    OutputsDir string  // 同上
    WDLPath    string  // WDL ファイルの絶対パス
}
```

3. Daemon は `InputsDir/inputs.json` を読み込む
4. WDL ファイルをパースし、タスク定義を取得する

### Step 4: Daemon → Docker（コンテナ実行）

**処理場所**: `wdl/task/task.go`, `wdl/task/runner.go`

#### 4a. 実行ディレクトリの作成

```
{baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}/{taskName}-{random}/
  ├── command          # 展開済みコマンドスクリプト
  ├── work/            # コンテナの /mnt/outputs にマウントされる
  ├── stdout.txt       # コンテナの標準出力
  ├── stderr.txt       # コンテナの標準エラー出力
  └── rc               # 終了コード
```

#### 4b. パスマッピング（PathMap）

`wdl/task/pathmap.go` にて、ホストパスとコンテナ内パスのマッピングを構築する。

| ホスト上のパス | コンテナ内のパス | マウントモード |
|---------------|-----------------|-------------|
| `{baseDir}/runs/{runID}/inputs/input.txt` | `/mnt/inputs/input.txt` | read-only |
| `{execDir}/command` | `/mnt/inputs/_command` | read-only |
| `{execDir}/work/` | `/mnt/outputs/` | read-write |

- ファイル名の衝突時は SHA256 の先頭 8 文字をプレフィックスとして付与する
- 入力ファイルは全て `/mnt/inputs/` 配下に read-only でマウントされる
- 出力先は `/mnt/outputs/`（= ホスト上の `{execDir}/work/`）

#### 4c. コマンド展開

`wdl/task/command.go` にて:

1. WDL の `command` セクションのテンプレートを評価（変数展開・関数呼出し）
2. コマンド内のホストパスをコンテナ内パスに置換
3. 作業ディレクトリ参照を `/mnt/outputs/` に置換
4. 完成したスクリプトを `{execDir}/command` に書き出す

#### 4d. Docker コンテナ実行

`wdl/task/runner.go` にて `DockerRunner.Run()` を呼び出す:

```go
RunRequest{
    Image:    "ubuntu:24.04",                    // runtime { docker: "..." } から
    Command:  ["bash", "/mnt/inputs/_command"],  // コマンドスクリプトを実行
    Binds:    [                                  // bind-mount リスト
        "host/input.txt:/mnt/inputs/input.txt:ro",
        "execDir/command:/mnt/inputs/_command:ro",
        "execDir/work:/mnt/outputs:rw",
    ],
    WorkDir:  "/mnt/outputs",
}
```

Docker SDK を使用してコンテナを作成・起動・待機・削除する。

#### 4e. 出力収集

`wdl/task/outputs.go` にて:

1. WDL の `output` セクションの各式を評価
2. コンテナ内パス (`/mnt/outputs/...`) をホストパス (`{execDir}/work/...`) に逆変換
3. 結果を `{outputsDir}/outputs.json` に書き出す

### Step 5: Daemon → Server（結果報告）

1. Daemon は `POST /tasks/{taskID}/result` で完了を通知する
2. Server はタスク状態を更新し、DAG スケジューラが次のタスクを解放する
3. 全タスク完了後、ワークフロー出力を `{baseDir}/runs/{runID}/outputs.json` に書き出す

### Step 6: Client が結果を取得

1. Client は `GET /workflows/{runID}/status` でポーリング（500ms 間隔）
2. 完了後 `GET /workflows/{runID}/outputs` で出力 JSON を取得
3. 標準出力に出力 JSON を表示する

---

## flowy-data ディレクトリ構造

```
flowy-data/
├── flowy.db                                    # SQLite データベース
└── runs/
    └── {runID}/
        ├── inputs/                             # ステージング領域（Client がコピー）
        │   ├── workflow.wdl
        │   └── input.txt
        ├── outputs/                            # （将来の使用のため予約）
        ├── tasks/
        │   └── {taskName}_{shardIndex}/
        │       ├── inputs.json                 # タスク入力（Server が書き出し）
        │       ├── outputs.json                # タスク出力（Daemon が書き出し）
        │       └── {taskName}-{random}/        # 実行ディレクトリ（Daemon が作成）
        │           ├── command
        │           ├── work/                   # Docker の /mnt/outputs
        │           │   └── (出力ファイル群)
        │           ├── stdout.txt
        │           ├── stderr.txt
        │           └── rc
        └── outputs.json                        # ワークフロー最終出力（Server が書き出し）
```

---

## API エンドポイント一覧

### Client → Server

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/workflows` | ワークフローを送信する |
| `GET` | `/workflows/{id}/status` | 実行状態を取得する |
| `GET` | `/workflows/{id}/outputs` | 出力 JSON を取得する |
| `POST` | `/tasks/run` | 単一タスクを直接実行する |
| `GET` | `/health` | ヘルスチェック |

### Daemon → Server

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/tasks/claim` | 保留中のタスクを取得する |
| `POST` | `/tasks/{id}/result` | タスク結果を報告する |
| `POST` | `/heartbeat` | ハートビートを送信する |

---

## 設定ファイル（flowy.yaml）

```yaml
base_dir: ./flowy-data           # 全データのルート（絶対パスに変換される）
db_driver: sqlite                # DB バックエンド
db_dsn: ""                       # DB 接続文字列（省略時はデフォルト）

server:
  listen_addr: ":8124"           # サーバのリッスンアドレス
  heartbeat_timeout_sec: 60      # Daemon のタイムアウト（秒）

daemon:
  server_url: "http://localhost:8124"
  heartbeat_sec: 30              # ハートビート間隔（秒）
  poll_timeout_sec: 30           # ロングポーリングタイムアウト（秒）
  workers: 4                     # 同時実行ワーカー数

client:
  server_url: "http://localhost:8124"
```

---

## 重要な設計上の特徴

1. **全パスが絶対パス**: Client のステージング時点で相対パスは絶対パスに変換され、以降は全て絶対パスで受け渡される
2. **共有 FS 前提**: Client・Server・Daemon が同一の `base_dir` にアクセスできる必要がある
3. **パス変換は 3 箇所で発生**:
   - Client: ユーザのパス → ステージング領域
   - Daemon: ホストパス → コンテナ内パス（`/mnt/inputs/`, `/mnt/outputs/`）
   - Daemon: コンテナ内パス → ホストパス（出力収集時）
4. **ファイル内容は API を経由しない**: HTTP API で送受信されるのはパス文字列のみ。実際のファイルは共有 FS 上の bind-mount を通じて直接読み書きされる
