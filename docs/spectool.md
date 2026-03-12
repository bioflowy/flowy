# spectool について

## spectool とは

[spectool](https://github.com/openwdl/spectool) は、WDL（Workflow Description Language）1.2 仕様に対するコンフォーマンステスト（準拠テスト）を実行するための公式ツールである。Rust で書かれており、WDL エンジンの仕様準拠度を自動的に検証できる。

## 仕組み

spectool は WDL 仕様リポジトリ内のテストケースを読み込み、指定されたコマンドテンプレートに従って WDL エンジンを呼び出し、その出力が期待される結果と一致するかを検証する。

```
WDL 仕様リポジトリ (github.com/openwdl/wdl)
    └── tests/
        └── (各テストケース: WDL ファイル + 入力 JSON + 期待出力)
                │
                ▼
          spectool が各テストに対して
          flowy run <wdl> --input <input> --task <task>
          を実行し、出力を期待値と比較
```

## flowy での使い方

### 前提条件

- Rust ツールチェーン（`cargo`）がインストールされていること
- Docker が利用可能であること

### spectool のインストール

```bash
make install-spectool
```

これは `cargo install` で `github.com/openwdl/spectool` をソースからビルドし、`~/.cargo/bin/spectool` にインストールする。

### テストの実行

```bash
make spectool
```

このコマンドは以下を順に実行する:

1. **ビルド**: `flowy`, `flowy-server`, `flowy-daemon` をビルド
2. **WDL 仕様クローン**: `github.com/openwdl/wdl` を `/tmp/wdl-spec` にクローン（または更新）
3. **サーバ起動**: `flowy-server` をバックグラウンドで起動
4. **デーモン起動**: `flowy-daemon` をバックグラウンドで起動
5. **テスト実行**: spectool を以下のコマンドテンプレートで実行:
   ```
   flowy run ~{path} --input ~{input} --task ~{target}
   ```
   - `~{path}`: テスト用 WDL ファイルのパス
   - `~{input}`: 入力 JSON ファイルのパス
   - `~{target}`: テスト対象タスク名
   - `--redirect-stdout` オプションで出力 JSON をキャプチャ
6. **サービス停止**: サーバとデーモンを停止

### 除外パターン

`spectool-exclude.txt` に記載されたテストパターンはスキップされる。現在の除外理由:

| カテゴリ | 理由 |
|---------|------|
| Object 型 | WDL 1.2 で非推奨。実装しない |
| Sub-workflow call | ワークフローからワークフローの呼び出し。未実装 |
| Deferred input defaults | 入力デフォルト値の遅延評価。未実装 |
| read_tsv (Object 返却) | Object 型に依存する stdlib 関数。Object 未実装のため |

### 現在の準拠状況

spectool-exclude.txt の先頭にベースライン情報が記載されている。

## Makefile ターゲット一覧

| ターゲット | 説明 |
|-----------|------|
| `make install-spectool` | spectool を Rust ソースからインストール |
| `make spectool` | 全ての準拠テストを実行 |
| `make build` | flowy バイナリのビルド |
| `make build-server` | flowy-server バイナリのビルド |
| `make build-daemon` | flowy-daemon バイナリのビルド |

## ディレクトリ構造

```
flowy/
├── Makefile                # spectool 統合ターゲット（68-109行目）
├── spectool-exclude.txt    # 除外テストパターンとベースライン情報
├── flowy-wrapper.sh        # spectool から呼び出されるラッパースクリプト
└── /tmp/
    └── wdl-spec/           # クローンされた WDL 仕様リポジトリ（.gitignore 対象）
```

## トラブルシューティング

- **spectool が見つからない**: `make install-spectool` を実行する
- **Docker エラー**: Docker デーモンが起動しているか確認する
- **ポート競合**: `flowy-server` がデフォルトで `:8124` を使用する。他のプロセスと競合していないか確認する
- **テスト失敗の調査**: 個別の WDL ファイルを `flowy run` で直接実行して出力を確認する
