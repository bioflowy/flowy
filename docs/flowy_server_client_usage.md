# flowy-server / flowy-client 利用ガイド

## 概要
`flowy-server` は WDL ワークフロー / タスクをリモート実行するための REST API サーバーです。`flowy-client` はそのエンドポイントに JSON リクエストを送信するための CLI で、ローカルの WDL ファイルと入力 JSON を指定して実行結果を取得できます。本書では開発環境で両者をビルドして利用する手順をまとめます。

## 前提条件
- Rust 1.82 以上 (`Cargo.toml` の `rust-version` に従う)
- `cargo` が PATH に通っていること
- (任意) Docker 実行を伴う WDL を動かす場合は Docker デーモンが起動していること

レポジトリをクローンしたルート (`/home/uehara/flowy`) を前提に説明します。

## ビルド手順
```bash
# 依存関係を取得しつつ両バイナリをビルド
cargo build --bin flowy-server --bin flowy-client
```
ビルドが完了すると `target/debug/flowy-server` と `target/debug/flowy-client` が生成されます。リリースビルドが必要な場合は `--release` を付与してください。

## flowy-server の起動
```bash
# デフォルトの 0.0.0.0:3030 で待ち受け
cargo run --bin flowy-server
```
- サーバーは `/api/v1/tasks` で POST リクエストを受け付けます。
- 各リクエストごとに一時ディレクトリ上でワークフロー / タスクを実行し、完了すると JSON で結果を返します。
- 標準出力レベルで `flowy-server listening on http://0.0.0.0:3030` が表示されれば起動成功です。

別のターミナルを開き、`flowy-server` を起動したまま `flowy-client` や `curl` でアクセスします。

## REST リクエスト形式
`POST /api/v1/tasks` に以下の JSON を送信します。
```json
{
  "wdl": "<WDL ファイルの中身>",
  "inputs": { /* WDL に対応する JSON 入力 */ },
  "options": {
    "task": "<任意: 直接実行する task 名>",
    "run_id": "<任意: 実行 ID>"
  }
}
```
- `wdl` にはファイル内容の文字列を入れます。
- `inputs` は WDL 名空間に揃えた JSON オブジェクトで、省略した場合は空オブジェクトとして扱われます。
- `options.task` を指定するとワークフローではなく該当タスクを直接実行します。ワークフローが存在する場合は未指定でワークフローが実行されます。
- レスポンスは `ExecuteResponse` として以下の構造で返ります。
```json
{
  "status": "ok",
  "outputs": { /* WDL 出力 */ },
  "stdout": "...",
  "stderr": "...",
  "duration_ms": 1234
}
```
エラー時は `status: "error"` と `message` を含む `ErrorResponse` が返るので、HTTP ステータスコードと併せて確認してください。

## flowy-client の使い方
`flowy-client` は `flowy` CLI と同じ形式で利用できます。
```bash
flowy-client run path/to/workflow.wdl \
  [-i path/to/inputs.json] \
  [-t task_name] \
  [-s http://localhost:3030] \
  [--basedir /shared/project]
```
主なオプション:
- `-i, --input`: JSON 入力ファイルのパス (省略時は空オブジェクト `{}` を送信)
- `-t, --task`: 文書内 task を直接実行する場合に指定
- `-s, --server`: `flowy-server` のベース URL (`/api/v1/tasks` は自動で付与)。省略時はローカル設定ファイルから読み込みます
- `--basedir`: `File` 型の相対パスを解決する基準ディレクトリ (省略時は実行時のカレントディレクトリ)
- `--debug`: クライアント側の詳細ログを有効化 (設定ファイルの `DEBUG` でも指定可)

`--basedir` で指定されたパスは `flowy-server` にそのまま送信され、サーバー側で `File` 入力の相対パスを解決する際の基準になります。クライアントとサーバーが同じファイルシステムを共有しているケース (NFS など) を想定しており、該当ディレクトリ以下に入力ファイルが存在する必要があります。

実行後は CLI 上で `status`, `duration_ms`, `outputs`, `stdout`, `stderr` が順に表示されます。終了コードも成功/失敗で変わるため、シェルスクリプトから扱う際は `$?` を確認可能です。

### デフォルトのサーバー URL
`flowy-client` は `~/.flowy` (TOML 形式) を設定ファイルとして参照し、既定のサーバー URL やデバッグ設定を取得します。例えばローカルサーバーとデバッグ出力を有効化する場合:
```bash
cat <<'TOML' > ~/.flowy
SERVER_URL = "http://localhost:3030"
DEBUG = true
TOML
```
`--server` オプションを指定すると、その値が即座に利用されるとともに同ファイルへ保存されるため、次回以降はオプションを省略できます。`DEBUG` キーが `true` の場合、`flowy-client --debug` を付けなくても詳細ログが出力されます。`--basedir` は CLI オプションで指定し、未指定時はクライアントのカレントディレクトリが自動的に送信されます。ファイルが存在しない、または `SERVER_URL` が設定されていない場合はエラーになります。

## 動作例
1. サーバーを起動:
   ```bash
   cargo run --bin flowy-server
   ```
   - `--debug` を付与するとサーバー側の詳細ログが有効になります (例: `cargo run --bin flowy-server -- --debug`)。`~/.flowy` に `DEBUG = true` を記述しておけばフラグなしでも同様に有効化されます。
2. 別ターミナルでサンプルワークフローを実行:
   ```bash
   flowy-client run examples/docker_simple.wdl \
     -i examples/docker_hello_inputs.json
   ```
   - `examples/docker_simple.wdl` には `docker_hello_workflow` が定義されており、`examples/docker_hello_inputs.json` が入力の例です。
   - WDL 内にワークフローが存在するため `--task` は不要です。

`curl` で直接叩きたい場合は以下が参考になります。
```bash
jq -n \
  --arg wdl "$(cat examples/simple_command.wdl)" \
  '{
    wdl: $wdl,
    inputs: {},
    options: {task: "test"}
  }' > request.json

curl -X POST http://localhost:3030/api/v1/tasks \
  -H 'Content-Type: application/json' \
  --data @request.json
```
上記では単一タスク `test` を直接実行する例です。`wdl` はファイル内容を 1 行化して埋め込んでいますが、実際には `jq -Rs` や専用スクリプトでエスケープすると安全です。

## トラブルシューティング
- **サーバーに接続できない**: ポート `3030` が空いていること、`flowy-server` プロセスが稼働していることを確認してください。
- **JSON パースエラー**: `flowy-client` では入力ファイルが正しい JSON か検証されます。`jq` などで事前に整形してください。
- **タスク名が見つからない**: `--task` に指定した名前は WDL 内の task 名と一致している必要があります。ワークフローを実行する場合は指定を省略してください。
- **Docker 実行が必要な WDL**: サーバー側で Docker が利用可能な設定になっているか確認し、必要に応じて `runtime` ブロックを適切に記述してください (現行の `Config` では非コンテナ実行がデフォルトです)。

### spec_tests での実行モード切り替え
WDL 仕様テストランナー (`spec_tests`) からも `flowy` (ローカル実行) と `flowy-client` (リモート実行) を選択できます。
```bash
cargo run --bin spec_tests -- \
  spec/wdl-1.2/SPEC.md \
  miniwdl/tests/spec_tests/data \
  --runner flowy-client \
  --client-server http://localhost:3030
```
`--runner flowy-client` を選ぶと内部的に `flowy-client run` を呼び出し、`--client-server` で指定した URL (省略時は `~/.flowy` の `SERVER_URL`) を利用します。既定値の `--runner flowy` では従来どおりローカルの `flowy` バイナリを用います。

## 参考
- REST API 型定義: `src/core/api.rs`
- サーバー実装: `src/bin/flowy-server.rs`
- クライアント実装: `src/bin/flowy-client.rs`
