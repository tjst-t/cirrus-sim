# Phase 3: 運用機能

## 目標
ホストプロファイル適用、バックエンドドレインが動作する。

## チェックリスト

### 1. awx-sim（AWX REST API互換）

- [x] 1.1 go.mod初期化、基本ディレクトリ構成
- [x] 1.2 インメモリ状態管理（JobTemplate, Job）
- [x] 1.3 ジョブテンプレートAPI (`POST/GET /api/v2/job_templates/`)
- [x] 1.4 ジョブ実行API (`POST /api/v2/job_templates/{id}/launch/`, `GET /api/v2/jobs/{id}/`)
- [x] 1.5 ジョブキャンセル (`POST /api/v2/jobs/{id}/cancel/`)
- [x] 1.6 ジョブ状態遷移（pending→running→successful/failed）タイミング制御
- [x] 1.7 コールバック設定・送信 (`POST /sim/config/callback`)
- [x] 1.8 管理API (`GET /sim/stats`, `POST /sim/reset`)
- [x] 1.9 cmd/main.go エントリポイント
- [x] 1.10 全テスト通過確認

### 2. netbox-sim（NetBox REST API互換）

- [x] 2.1 go.mod初期化、基本ディレクトリ構成
- [x] 2.2 インメモリ状態管理（Site, Rack, Device, Location）
- [x] 2.3 サイトAPI (`GET /api/dcim/sites/`)
- [x] 2.4 ラックAPI (`GET /api/dcim/racks/`, フィルタ: site_id)
- [x] 2.5 デバイスAPI (`GET /api/dcim/devices/`, フィルタ: rack_id, role)
- [x] 2.6 バルクロード (`POST /sim/bulk-load`)
- [x] 2.7 管理API (`GET /sim/stats`, `POST /sim/reset`)
- [x] 2.8 cmd/main.go エントリポイント
- [x] 2.9 全テスト通過確認

### 3. storage-sim追加: ストレージマイグレーション

- [x] 3.1 マイグレーション開始 (`POST /api/v1/migrations`)
- [x] 3.2 マイグレーション状態取得 (`GET /api/v1/migrations/{id}`)
- [x] 3.3 マイグレーションキャンセル (`DELETE /api/v1/migrations/{id}`)
- [x] 3.4 非同期進捗追跡（preparing→transferring→finishing→completed）
- [x] 3.5 完了時の移行元削除・移行先登録
- [x] 3.6 全テスト通過確認

### 4. common追加: 障害注入エンジン

- [x] 4.1 障害注入ライブラリ（pkg/fault/）
- [x] 4.2 障害タイプ: error, delay, timeout, partial_failure
- [x] 4.3 トリガー: probabilistic, count, time
- [x] 4.4 障害注入API (`POST/GET/DELETE /api/v1/faults`)
- [x] 4.5 全テスト通過確認

### 5. Phase完了

- [x] 5.1 `make test` で全テスト通過
- [x] 5.2 `make lint` でlint通過
- [x] 5.3 設計原則のセルフレビュー
- [x] 5.4 コミット

## 完了メモ

### 実装判断
- awx-simのジョブ実行はgoroutine + time.AfterFuncで非同期にタイミング制御
- netbox-simはNetBox API互換のレスポンス形式（count + results、statusはオブジェクト）
- ストレージマイグレーションはpreparing→transferring→finishing→completedの4フェーズを非同期goroutineで進行
- 障害注入エンジンはpkg/fault/として他シミュレータからimport可能な設計

### 想定外だった点
- 障害注入のcountトリガーでCallCount（全呼び出し回数）とHitCount（実際に発火した回数）を分離する必要があった

### 次Phaseへの申し送り
- Phase 4: データジェネレータ、状態スナップショット/リストア、OVN-IC、load-gen、環境定義
