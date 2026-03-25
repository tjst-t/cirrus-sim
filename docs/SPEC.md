# Cirrus-Sim シミュレータ仕様書

本ファイルは要約版です。各シミュレータのAPI定義、データ構造、状態遷移、シミュレーション設定等の詳細は [docs/spec/](spec/) を参照してください。

| 詳細仕様 | 内容 |
|---------|------|
| [spec/01-overview.md](spec/01-overview.md) | 概要、プロトコル方針、リポジトリ構成、設計原則 |
| [spec/02-common.md](spec/02-common.md) | 共通ライブラリ（障害注入、イベントログ、状態管理、データジェネレータ） |
| [spec/03-libvirt-sim.md](spec/03-libvirt-sim.md) | libvirtシミュレータ（RPC実装、状態遷移、マイグレーション） |
| [spec/04-ovn-sim.md](spec/04-ovn-sim.md) | OVNシミュレータ（OVSDBプロトコル、スキーマ、参照整合性） |
| [spec/05-storage-sim.md](spec/05-storage-sim.md) | ストレージシミュレータ（Cirrus Storage API、容量追跡） |
| [spec/06-awx-sim.md](spec/06-awx-sim.md) | AWXシミュレータ（ジョブ実行、コールバック） |
| [spec/07-netbox-sim.md](spec/07-netbox-sim.md) | NetBoxシミュレータ（DCIM/CMDB） |
| [spec/08-load-gen.md](spec/08-load-gen.md) | 負荷ジェネレータ（ワークロード定義） |
| [spec/09-docker-compose.md](spec/09-docker-compose.md) | docker-compose構成 |
| [spec/10-implementation-priority.md](spec/10-implementation-priority.md) | 実装の優先順位（Phase 1〜4） |

---

## プロトコル方針

```
libvirt:   Cirrus → libvirt RPC (XDR)         → libvirtd (本番) / libvirt-sim (開発)
OVN:       Cirrus → OVSDB (JSON-RPC)          → ovsdb-server (本番) / ovn-sim (開発)
Storage:   Cirrus → Cirrus Storage API (REST)  → storage-agent (本番) / storage-sim (開発)
AWX:       Cirrus → AWX REST API               → AWX (本番) / awx-sim (開発)
NetBox:    Cirrus → NetBox REST API            → NetBox (本番) / netbox-sim (開発)
```

## 設計原則

- 全シミュレータが本番と同一プロトコルで通信
- 1プロセスで数千ホスト分をシミュレーション可能（マルチホストモード）
- 全シミュレータがインメモリで動作し、外部DBやストレージ不要
- 障害注入・レイテンシ注入・イベントログを全シミュレータで共通利用
- 状態のスナップショット・リストアによるテスト再現性の確保

## 実装の優先順位

1. **Phase 1**: libvirt-sim (ドメインCRUD) + storage-sim (ボリュームCRUD) + common (イベントログ)
2. **Phase 2**: ovn-sim (OVSDB) + ライブマイグレーション + スナップショット/クローン
3. **Phase 3**: awx-sim + netbox-sim + ストレージマイグレーション + 障害注入
4. **Phase 4**: データジェネレータ + load-gen + OVN-IC + 大規模環境定義
