# flowy

## What This Is

WDL (Workflow Description Language) 1.2 ワークフローエンジンのGo実装。
miniwdl（Python製WDLエンジン）を参考にGoへ移植し、WDL 1.2のパース・型チェック・タスク/ワークフロー実行をサポートする。
v1はCLIツールとしてコア機能を提供し、後続マイルストーンでREST APIサーバとクライアントに分離してプロジェクト管理機能を追加する。

## Core Value

WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること。

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] WDL 1.2 文法の完全なパーサ（AST生成）
- [ ] WDL 型システム（Primitive, Compound, Optional, File, Directory等）
- [ ] 式の評価（演算子、関数呼び出し、型強制）
- [ ] タスク定義の読み込みと入力バインディング
- [ ] コマンドテンプレートの展開（`~{}` および `${}` プレースホルダ）
- [ ] Dockerコンテナでのタスク実行
- [ ] ワークフローの依存関係解析と実行順制御
- [ ] scatter / if（conditional）のサポート
- [ ] 標準ライブラリ関数の実装（stdlib）
- [ ] CLIツール（`flowy run`, `flowy check` 相当）

### Out of Scope (v1)

- HTTP(S) import解決 — まずローカルファイルのみ
- ユーザ認証 / マルチユーザ管理 — v2以降
- REST APIサーバ / クライアント分離 — v2以降
- プロジェクト単位の実行履歴・出力管理 — v2以降
- lint / zip 機能 — 後回し
- Docker以外のバックエンド（Podman, Singularity等）— 後回し
- キャッシュ機能 — 後回し

## Context

- **参照実装**: `../miniwdl`（Pythonで約2万行）がチェックアウト済み
- **WDLバージョン**: 1.2のみサポート。古いバージョン（draft-2, 1.0, 1.1）は対象外
- **Goモジュール名**: `flowy`
- **パーサ**: miniwdlはlark（Python PEG）パーサを使用。Go移植ではgoyacc or participle等を検討
- **コンテナ**: タスク実行はDocker（Docker SDK for Go）
- **主要ファイル**: `../miniwdl/WDL/_grammar.py`（文法）, `Tree.py`（AST）, `Type.py`, `Value.py`, `Expr.py`, `runtime/`

## Constraints

- **Tech stack**: Go 1.21+、Docker SDK for Go
- **WDL version**: 1.2のみ（古いバージョン対応は不要）
- **Import**: ローカルファイルのみ（HTTP importは除外）
- **互換性**: miniwdlのテストスイートを参考に動作を合わせる

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| WDL 1.2のみ対象 | スコープを絞って確実に実装する | — Pending |
| v1はCLIのみ | まずコアエンジンを完成させてから分離 | — Pending |
| SQLiteで管理（v2以降） | 依存なし・シンプル・単一サーバ向け | — Pending |
| REST API + CLI（v2以降） | 標準的なWebAPI構成 | — Pending |

---
*Last updated: 2026-03-10 after initialization*
