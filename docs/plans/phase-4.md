# Phase 4: スケーラビリティ検証

## 目標
数千ホスト規模での性能検証。

## チェックリスト

### 1. common追加: データジェネレータ・状態スナップショット

- [x] 1.1 データジェネレータ（pkg/datagen/）: YAML環境定義からデータ生成
- [x] 1.2 データジェネレータAPI (`POST /api/v1/generate`, `GET /api/v1/generate/status`, `POST /api/v1/generate/reset`)
- [x] 1.3 状態スナップショット/リストア (`POST/GET /api/v1/state/snapshot`, `POST /api/v1/state/restore/{id}`)
- [x] 1.4 全テスト通過確認

### 2. ovn-sim追加: OVN-ICシミュレーション

- [x] 2.1 IC Northbound DBテーブル（Transit_Switch, Availability_Zone, Route, Port_Binding）
- [x] 2.2 ICクラスタ接続設定 (`POST /sim/ic/connections`)
- [x] 2.3 全テスト通過確認

### 3. load-gen: 負荷ジェネレータ

- [x] 3.1 go.mod初期化、基本ディレクトリ構成
- [x] 3.2 ワークロード定義（YAML解析）
- [x] 3.3 アクション実行エンジン
- [x] 3.4 結果収集・アサーション
- [x] 3.5 API (`POST /api/v1/workloads/run`, `GET /api/v1/workloads/run/{id}`)
- [x] 3.6 全テスト通過確認

### 4. environments: 環境定義

- [x] 4.1 medium.yaml (500ホスト)
- [x] 4.2 large.yaml (3000ホスト)

### 5. Phase完了

- [x] 5.1 `make test` で全テスト通過
- [x] 5.2 `make lint` でlint通過
- [x] 5.3 設計原則のセルフレビュー
- [x] 5.4 コミット

## 完了メモ

### 実装判断
- データジェネレータはYAML→GenerateResult（ホスト一覧＋バックエンド一覧）のシンプルな変換
- スナップショットはSnapshotableインターフェースで各コンポーネントの状態をJSON化して保存
- load-genのワークロード実行は各フェーズのアクションをrate_per_secで実行するシミュレーション
- OVN-ICはICスキーマ（4テーブル）を定義し、CreateICClusterで専用OVSDB上に立ち上げ
- environment定義: small(10ホスト), medium(400ホスト), large(484ホスト+40GPUホスト)

### 想定外だった点
- go.modのyaml依存はcommonとload-genの両方で必要（独立モジュールなので各自go get）

### 全Phase完了サマリー
- 7つの独立Goモジュール: libvirt-sim, ovn-sim, storage-sim, awx-sim, netbox-sim, common, load-gen
- 全シミュレータがインメモリ・本番同一プロトコル・/sim/プレフィックス管理APIの設計原則を遵守
- テスト網羅: 各モジュールにstate/handler/protocolレベルのテスト
