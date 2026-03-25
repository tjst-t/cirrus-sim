# Cirrus-Sim

Cirrus IaaS開発用のシミュレータ群。本番と同一プロトコルで通信し、実ハードウェアなしにCirrusの全機能を開発・テストする。

## プロジェクト構成

```
cirrus-sim/
├── common/          共有ライブラリ（障害注入、イベントログ、データ生成）
├── libvirt-sim/     libvirt RPCプロトコル（XDR over TCP, port 16509+）
├── ovn-sim/         OVSDBプロトコル（JSON-RPC over TCP, port 6641+）
├── storage-sim/     Cirrus Storage API（REST, port 8500）
├── awx-sim/         AWX REST API互換（port 8300）
├── netbox-sim/      NetBox REST API互換（port 8400）
├── load-gen/        負荷ジェネレータ
└── environments/    環境定義YAML
```

## 技術スタック

- 言語: Go（全シミュレータ共通）
- ビルド: `make build` （各シミュレータ個別は `make build-libvirt-sim` 等）
- テスト: `make test` （全体）、`make test-libvirt-sim` （個別）
- Lint: `make lint` （golangci-lint）
- コンテナ: `docker-compose up -d`
- Go modules: 各シミュレータは独立モジュール、commonは共有モジュール

## 設計原則（厳守）

- 全シミュレータは本番と同一プロトコルを喋る。独自プロトコルへの差し替え禁止
- 全状態はインメモリ。外部DB依存禁止
- 1プロセスで数千ホスト/バックエンドをシミュレーション可能にする
- 障害注入・イベントログはcommonの共有ライブラリを使う
- シミュレータ固有の管理APIは `/sim/` プレフィックスで本番APIと分離

## コーディング規約

- エラーは必ずラップして返す: `fmt.Errorf("operation failed: %w", err)`
- テーブル駆動テストを基本とする
- 公開APIは必ずGoDocコメントを書く
- ログはstructured logging（slog）を使う
- context.Contextを第一引数に取る

## 実装の優先順位

Phase 1（最小開発環境）から順に実装する:
1. libvirt-sim: ホスト登録、ドメインCRUD、状態遷移、リソース追跡
2. storage-sim: ボリュームCRUD、エクスポート/アンエクスポート、容量追跡
3. common: イベントログ

詳細仕様は `docs/SPEC.md` を参照。アーキテクチャは `docs/ARCHITECTURE.md` を参照。
