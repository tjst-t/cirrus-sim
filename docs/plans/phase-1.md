# Phase 1: 最小開発環境

## 目標
VM作成→ボリュームアタッチ→起動→停止→削除の基本フローが通る。

## チェックリスト

### 1. storage-sim（REST APIなので先に実装）

- [x] 1.1 go.mod初期化、基本ディレクトリ構成
- [x] 1.2 インメモリ状態管理（Backend, Volume）
- [x] 1.3 管理API: バックエンド登録・一覧・状態変更・リセット (`/sim/`)
- [x] 1.4 Storage API: バックエンド情報 (`GET /backend/info`, `/backend/health`)
- [x] 1.5 Storage API: ボリュームCRUD (`POST/GET/DELETE /volumes`, `PUT /volumes/{id}/extend`)
- [x] 1.6 Storage API: エクスポート/アンエクスポート (`POST/DELETE /volumes/{id}/export`)
- [x] 1.7 容量追跡（thin provisioning、overprovisioning）
- [x] 1.8 cmd/main.go エントリポイント、サーバ起動
- [x] 1.9 全テスト通過確認

### 2. common（イベントログ）

- [x] 2.1 go.mod初期化、基本ディレクトリ構成
- [x] 2.2 イベントログ: データ構造とインメモリストア
- [x] 2.3 イベントログAPI (`GET/DELETE /api/v1/events`)
- [x] 2.4 cmd/main.go エントリポイント
- [x] 2.5 全テスト通過確認

### 3. libvirt-sim（XDRプロトコル、段階的に実装）

- [x] 3.1 go.mod初期化、基本ディレクトリ構成
- [x] 3.2 インメモリ状態管理（Host, Domain, リソース追跡）
- [x] 3.3 管理API: ホスト登録・一覧・情報・状態変更・リセット (`/sim/`)
- [x] 3.4 libvirt RPCフレームワーク: TCPリスナー、RPCヘッダパース、XDRエンコード/デコード
- [x] 3.5 接続管理RPC: CONNECT_OPEN, CONNECT_CLOSE, GET_HOSTNAME, GET_VERSION, GET_CAPABILITIES
- [x] 3.6 ホスト情報RPC: NODE_GET_INFO, NODE_GET_CPU_STATS, NODE_GET_MEMORY_STATS, NODE_GET_FREE_MEMORY
- [x] 3.7 ドメイン管理RPC: DEFINE_XML, CREATE, DESTROY, SHUTDOWN, REBOOT, SUSPEND, RESUME, UNDEFINE
- [x] 3.8 ドメイン情報RPC: GET_XML_DESC, GET_INFO, GET_STATE, LIST_ALL_DOMAINS, LOOKUP_BY_UUID, LOOKUP_BY_NAME
- [x] 3.9 ドメイン状態遷移の正確な実装（エラーコード含む）
- [x] 3.10 リソース消費追跡（vCPU、メモリ、GPU）
- [x] 3.11 GET_ALL_DOMAIN_STATS RPC
- [x] 3.12 cmd/main.go エントリポイント
- [x] 3.13 全テスト通過確認

### 4. Phase完了

- [x] 4.1 `make test` で全テスト通過
- [x] 4.2 `make lint` でlint通過
- [x] 4.3 設計原則のセルフレビュー
- [x] 4.4 コミット

## 完了メモ

### 実装判断
- storage-simを先に実装（REST APIで最もシンプル）、commonのeventlogは並行して実装
- libvirt-simではdigitalocean/go-libvirtライブラリを依存に追加し、正確なプロシージャ番号定数を参照
- XDRエンコード/デコードは標準ライブラリのみで自前実装（外部XDRライブラリは不要なほどシンプル）
- Go 1.22のServeMuxメソッドルーティングを活用（外部ルータ不要）

### 想定外だった点
- golangci-lintのerrcheckがテストコード内のエラー無視にも厳格。全てのエラー戻り値を検査するよう修正

### 次Phaseへの申し送り
- Phase 2ではovn-sim（OVSDBプロトコル）、libvirt-simのライブマイグレーション、storage-simのスナップショット/クローンを実装
- libvirt-simのイベント通知（DOMAIN_EVENT_LIFECYCLE）もPhase 2スコープ
- eventlogライブラリは各シミュレータからHTTP経由で利用する設計（go.modの相互依存を避ける）
