# daemon-flowy を用いたワークフロー実行手順

## 前提
- `cargo build` 済みで `target/debug/` にバイナリが生成されていること。
- 同一ホスト上で `flowy-server` と `daemon-flowy` を実行する想定です。

## 1. サーバーを起動
ジョブキュー API を含む REST サーバーを立ち上げます。

```bash
cargo run --bin flowy-server -- --debug
```

起動後 `http://0.0.0.0:3030` で待受します。

## 2. デーモンを起動
ワーカー ID やポーリング間隔などはオプションで設定できます。以下は最小構成の例です。

```bash
cargo run --bin daemon-flowy -- \
  --server http://localhost:3030 \
  --worker-id worker-1 \
  --poll 5 \
  --heartbeat 60 \
  --max-jobs 1 \
  --debug
```

起動すると `daemon-flowy` が定期的にサーバーへジョブを問い合わせ、ハートビートを送信します。

## 3. ジョブを投入 (flowy-client)
`flowy-client` は常にジョブキュー経由で実行します。以下はサンプル:

```bash
cargo run --bin flowy-client -- run examples/docker_simple.wdl \
  -i examples/docker_hello_inputs.json \
  --server http://localhost:3030 \
  --basedir $(pwd)
```

- `--queue` フラグは後方互換性のために残っていますが、指定の有無に関わらず `/api/v1/jobs` にリクエストを登録し、`daemon-flowy` が実行するまでポーリングします。
- `--basedir` は `File` 入力の相対パス解決に使用されます。

## 4. 実行結果の確認
`flowy-client` はジョブ完了まで待機し、終了時に `status`, `duration_ms`, `outputs`, `stdout`, `stderr` を表示します。途中で `run_id: <ID>` が表示されるので、必要に応じて REST API 経由で状態を確認できます。

### 手動でジョブ状態を確認する場合

```bash
curl http://localhost:3030/api/v1/jobs/<run_id>
```

レスポンス例:

```json
{
  "run_id": "a1b2c3",
  "state": "succeeded",
  "worker_id": "worker-1",
  "result": {
    "type": "success",
    "response": {
      "status": "ok",
      "outputs": { ... },
      "stdout": "...",
      "stderr": "...",
      "duration_ms": 1234
    }
  }
}
```

## 補足
- 現状のジョブストアはメモリ常駐です。サーバー再起動でジョブ情報は失われます。
- キャンセル、再キュー、永続化などの高度な機能は未実装のため、必要に応じて追加してください。
