# Phase 2: ネットワークとマイグレーション

## 目標
VMにネットワーク接続、ライブマイグレーションが動作する。

## チェックリスト

### 1. ovn-sim（OVSDBプロトコル基盤）

- [x] 1.1 go.mod初期化、基本ディレクトリ構成
- [x] 1.2 OVSDBプロトコル: JSON-RPC over TCP フレームワーク
- [x] 1.3 インメモリOVSDBストア（テーブル、行、UUID管理）
- [x] 1.4 RPC: list_dbs, get_schema, echo
- [x] 1.5 RPC: transact（insert, select, update, delete, mutate）
- [x] 1.6 RPC: monitor / monitor_cancel（変更通知）
- [x] 1.7 OVN Northbound スキーマ対応（主要テーブル13種）
- [x] 1.8 参照整合性（strong/weak参照）
- [x] 1.9 管理API: クラスタ登録・ポート状態制御 (`/sim/`)
- [x] 1.10 cmd/main.go エントリポイント
- [x] 1.11 全テスト通過確認（35テスト）

### 2. libvirt-sim追加: ライブマイグレーション・イベント

- [x] 2.1 マイグレーションRPC: MIGRATE_PREPARE3_PARAMS, PERFORM3_PARAMS, FINISH3_PARAMS, CONFIRM3_PARAMS
- [x] 2.2 マイグレーション速度設定: GET_MAX_SPEED, SET_MAX_SPEED
- [x] 2.3 マイグレーション前提条件チェック
- [x] 2.4 イベント通知: CONNECT_DOMAIN_EVENT_REGISTER_ANY, DEREGISTER_ANY
- [x] 2.5 ライフサイクルイベントのpush通知
- [x] 2.6 全テスト通過確認

### 3. storage-sim追加: スナップショット・クローン

- [x] 3.1 スナップショットCRUD
- [x] 3.2 クローン (`POST /snapshots/{id}/clone`)
- [x] 3.3 フラット化 (`POST /volumes/{id}/flatten`, `GET /operations/{id}`)
- [x] 3.4 依存関係管理（parent_snapshot_id, child_clones）
- [x] 3.5 全テスト通過確認

### 4. Phase完了

- [x] 4.1 `make test` で全テスト通過
- [x] 4.2 `make lint` でlint通過
- [x] 4.3 設計原則のセルフレビュー
- [x] 4.4 コミット

## 完了メモ

### 実装判断
- ovn-simのOVSDBストアはmap[string]map[string]Row（テーブル→UUID→行）のシンプルな構造
- OVN NBスキーマは簡略化して13テーブルを定義（get_schemaで返す）
- transactは原子性を保証（スナップショットベースのロールバック）
- libvirt-simのマイグレーションはPhase 2では同期的（ブロッキング）に実装。非同期化はPhase 4でスケーラビリティ検証時に必要なら対応
- イベント通知はEventBus経由で全登録クライアントにpush

### 想定外だった点
- ovn-simのwhere条件マッチングでOVSDB特有のデータ型（set, map, uuid wrapper）の比較が必要
- libvirt-simのイベント通知をXDRでエンコードしてMESSAGEタイプで送信する実装が既存のrequest/replyフレームワークと異なるパターン

### 次Phaseへの申し送り
- Phase 3: awx-sim、netbox-sim、ストレージマイグレーション、障害注入エンジン
- ovn-simの参照整合性は基本的なチェックのみ。Phase 3以降で必要に応じて強化
