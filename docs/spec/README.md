# Cirrus-Sim シミュレータ仕様書

完全版の仕様書を分割して管理しています。

| ファイル | 内容 |
|---------|------|
| [01-overview.md](01-overview.md) | 概要、プロトコル方針、リポジトリ構成、設計原則 |
| [02-common.md](02-common.md) | 共通ライブラリ（障害注入、イベントログ、状態管理、データジェネレータ） |
| [03-libvirt-sim.md](03-libvirt-sim.md) | libvirtシミュレータ（RPC実装、状態遷移、マイグレーション） |
| [04-ovn-sim.md](04-ovn-sim.md) | OVNシミュレータ（OVSDBプロトコル、スキーマ、参照整合性） |
| [05-storage-sim.md](05-storage-sim.md) | ストレージシミュレータ（Cirrus Storage API、容量追跡） |
| [06-awx-sim.md](06-awx-sim.md) | AWXシミュレータ（ジョブ実行、コールバック） |
| [07-netbox-sim.md](07-netbox-sim.md) | NetBoxシミュレータ（DCIM/CMDB） |
| [08-load-gen.md](08-load-gen.md) | 負荷ジェネレータ（ワークロード定義） |
| [09-docker-compose.md](09-docker-compose.md) | docker-compose構成 |
| [10-implementation-priority.md](10-implementation-priority.md) | 実装の優先順位（Phase 1〜4） |
