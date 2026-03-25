# Cirrus-Sim シミュレータ仕様書

## 1. 概要

### 1.1 目的

Cirrus IaaSの開発・テストに必要な外部依存システムのシミュレータ群。Cirrusが本物の外部システムと通信するのと同じプロトコルで通信し、実際のハードウェアやインフラなしにCirrusの全機能を開発・テストできるようにする。

### 1.2 プロトコル方針

全シミュレータは本番環境と同一のプロトコルを喋る。Cirrus本体のコードは接続先エンドポイントの設定変更のみでシミュレータと本番を切り替えられる。

```
libvirt:   Cirrus → libvirt RPC (XDR)         → libvirtd (本番) / libvirt-sim (開発)
OVN:       Cirrus → OVSDB (JSON-RPC)          → ovsdb-server (本番) / ovn-sim (開発)
Storage:   Cirrus → Cirrus Storage API (REST)  → storage-agent (本番) / storage-sim (開発)
AWX:       Cirrus → AWX REST API               → AWX (本番) / awx-sim (開発)
NetBox:    Cirrus → NetBox REST API            → NetBox (本番) / netbox-sim (開発)
```

ストレージのみCirrus独自のREST APIを定義する。libvirtやOVNには業界標準プロトコルがあるが、ストレージにはボリューム管理の統一プロトコルが存在しないため。本番ではバックエンドごとのstorage-agent（Ceph agent、NFS agent等）がこのAPIを実装し、内部でlibrbd/NFS操作等に変換する。

詳細な仕様（API定義、データ構造、状態遷移、シミュレーション設定等）はローカルの `cirrus-sim-spec.md` を参照。本ファイルはその要約版。

### 1.3 設計原則

- 全シミュレータが本番と同一プロトコルで通信
- 1プロセスで数千ホスト分をシミュレーション可能（マルチホストモード）
- 全シミュレータがインメモリで動作し、外部DBやストレージ不要
- 障害注入・レイテンシ注入・イベントログを全シミュレータで共通利用
- 状態のスナップショット・リストアによるテスト再現性の確保

## 2. 共通ライブラリ (common/)

障害注入、イベントログ、状態管理、データジェネレータの4機能を提供。全シミュレータから共有ライブラリとして利用。REST API (ポート8000) で管理。

## 3. libvirt シミュレータ

- **プロトコル**: libvirt RPC (XDR over TCP, ポート16509+)
- **マルチホスト**: ホストごとに個別TCPポート
- **実装範囲**: 約25 RPCプロシージャ（接続管理、ドメインCRUD、状態遷移、リソース追跡、ライブマイグレーション、イベント通知）
- **管理API**: REST (ポート8100) でホスト登録・設定

## 4. OVN シミュレータ

- **プロトコル**: OVSDB (RFC 7047, JSON-RPC over TCP, ポート6641+)
- **スキーマ**: OVN実物の ovn-nb.ovsschema を使用
- **実装範囲**: list_dbs, get_schema, transact (insert/select/update/delete/mutate/wait), monitor, echo
- **参照整合性**: strong/weak参照をスキーマに基づき強制
- **OVN-IC**: マルチクラスタモード対応
- **管理API**: REST (ポート8200) でクラスタ登録・ポート状態制御

## 5. ストレージシミュレータ

- **プロトコル**: Cirrus Storage API (REST, ポート8500)
- **マルチバックエンド**: X-Backend-Id ヘッダで識別
- **機能**: ボリュームCRUD、エクスポート/アンエクスポート、スナップショット、クローン、フラット化、ストレージマイグレーション
- **容量追跡**: シンプロビジョニング対応（allocated vs consumed）
- **管理API**: /sim/ プレフィックスでバックエンド登録・設定

## 6. AWX シミュレータ

- **プロトコル**: AWX REST API互換 (ポート8300)
- **機能**: ジョブテンプレート登録、ジョブ起動、状態遷移シミュレーション、コールバック

## 7. NetBox シミュレータ

- **プロトコル**: NetBox REST API互換 (ポート8400)
- **機能**: サイト/ラック/デバイスの階層、バルクデータ投入

## 8. 負荷ジェネレータ

- YAML定義のワークロード（フェーズ、アクション種類、レート、アサーション）
- Cirrus本体に対してリクエストを発行し、メトリクスを収集

## 9. 実装の優先順位

1. **Phase 1**: libvirt-sim (ドメインCRUD) + storage-sim (ボリュームCRUD) + common (イベントログ)
2. **Phase 2**: ovn-sim (OVSDB) + ライブマイグレーション + スナップショット/クローン
3. **Phase 3**: awx-sim + netbox-sim + ストレージマイグレーション + 障害注入
4. **Phase 4**: データジェネレータ + load-gen + OVN-IC + 大規模環境定義
