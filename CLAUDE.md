# Cirrus-Sim

Cirrus IaaS開発用のシミュレータ群。本番と同一プロトコルで通信し、実ハードウェアなしにCirrusの全機能を開発・テストする。

## プロジェクト構成

```
cirrus-sim/
├── common/          共有ライブラリ（障害注入、イベントログ、データ生成）
├── libvirt-sim/     libvirt RPCプロトコル（XDR over TCP）
├── ovn-sim/         OVSDBプロトコル（JSON-RPC over TCP）
├── storage-sim/     Cirrus Storage API（REST）
├── awx-sim/         AWX REST API v2互換
├── netbox-sim/      NetBox REST API v4互換
├── postgres/        組み込みPostgreSQL（Cirrus本体のDB）
├── webui/           ダッシュボード Web UI
├── load-gen/        負荷ジェネレータ
├── cmd/cirrus-sim/  統一バイナリ（全コンポーネントを1プロセスで起動）
└── environments/    環境定義YAML
```

## 技術スタック

- 言語: Go（全コンポーネント共通）
- ビルド: `make build`（各シミュレータ個別は `make build-libvirt-sim` 等）
- テスト: `make test`（全体）、`make test-libvirt-sim`（個別）
- Lint: `make lint`（golangci-lint）
- 起動: `make serve`（portmanでポート確保→ビルド→環境シード→起動）
- 停止: `make stop`
- デプロイ: `make deploy`（ビルドして /usr/local/bin にインストール）
- コンテナ: `docker-compose up -d`
- Go modules: 各シミュレータは独立モジュール、commonは共有モジュール

## 設計原則（厳守）

- 全シミュレータは本番と同一プロトコルを喋る。独自プロトコルへの差し替え禁止
- シミュレータの全状態はインメモリ。外部DB依存禁止（PostgreSQLはCirrus本体用であり、シミュレータの状態管理には使わない）
- 1プロセスで数千ホスト/バックエンドをシミュレーション可能にする
- 障害注入・イベントログはcommonの共有ライブラリを使う
- シミュレータ固有の管理APIは `/sim/` プレフィックスで本番APIと分離
- シミュレータはCirrusの状態（ホストのonline/maintenance等）を管理しない。Cirrusが管理するものはCirrus側に任せる
- シミュレータ間の直接連携は行わない。Cirrusが各シミュレータを独立に操作する

## コーディング規約

- エラーは必ずラップして返す: `fmt.Errorf("operation failed: %w", err)`
- テーブル駆動テストを基本とする
- 公開APIは必ずGoDocコメントを書く
- ログはstructured logging（slog）を使う
- context.Contextを第一引数に取る

## ロードマップ

今後の実装計画は `docs/ROADMAP.md` を参照。Phase 1〜5は完了済み。次はPhase 6（CLIツール・障害注入統合）。

## 仕様・設計ドキュメント

- ロードマップ: `docs/ROADMAP.md`
- アーキテクチャ: `docs/ARCHITECTURE.md`
- 詳細仕様: `docs/SPEC.md`（各シミュレータの仕様は `docs/spec/` 配下）
- Phase別計画（履歴）: `docs/plans/`
