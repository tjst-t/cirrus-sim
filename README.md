# Cirrus-Sim

Simulator suite for [Cirrus IaaS](https://github.com/tjst-t/cirrus) development. Each simulator speaks the same protocol as its production counterpart, enabling full-stack IaaS development and testing without physical infrastructure.

## Components

| Component | Protocol | Description |
|-----------|----------|-------------|
| libvirt-sim | libvirt RPC (XDR/TCP) | Compute host simulation (VM lifecycle, migration, resources) |
| ovn-sim | OVSDB (JSON-RPC/TCP) | OVN Northbound DB (virtual networking) |
| storage-sim | Cirrus Storage API (REST) | Block storage backend (volumes, snapshots, clones) |
| awx-sim | AWX REST API v2 | Ansible AWX job execution |
| netbox-sim | NetBox REST API v4 | DCIM/CMDB (physical topology, failure domains) |
| postgres | PostgreSQL (embedded) | Cirrus本体のデータベース |
| common | REST | Shared services (fault injection, event log, data generator) |
| dashboard | HTTP | Operations dashboard (Web UI) |

## Quick Start

### Prerequisites

- Go 1.22+
- [portman](https://github.com/tjst-t/port-manager) (port management)

### Start all

```bash
# Build and start with small environment (10 hosts, 1 OVN cluster, 1 storage backend, PostgreSQL)
make serve

# Stop
make stop

# View logs
make logs
```

`make serve` は以下を自動で行います:

1. portman でポートを確保（管理API + ホストRPC + OVSDB + PostgreSQLの全ポート）
2. unified binary をビルド
3. PostgreSQL を組み込み起動（Cirrus用DB自動作成）
4. `environments/small.yaml` からホスト/クラスタ/バックエンド/物理トポロジをシード
5. バックグラウンドで起動

起動後、ダッシュボードのURLがターミナルに表示されます。

### Install binary

```bash
# Build and install to /usr/local/bin
make deploy
```

### Docker Compose (alternative)

```bash
docker-compose up -d
```

## Cirrus からの接続方法

### PostgreSQL

起動時にCirrus用のデータベースが自動作成されます。接続URLは起動バナーに表示されます。

```
postgresql://cirrus:cirrus@localhost:<POSTGRES_PORT>/cirrus
```

環境変数でカスタマイズ可能:

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `POSTGRES_PORT` | 5432 | PostgreSQL listen port |
| `POSTGRES_DATABASE` | cirrus | データベース名 |
| `POSTGRES_USERNAME` | cirrus | ユーザ名 |
| `POSTGRES_PASSWORD` | cirrus | パスワード |
| `POSTGRES_DATA_DIR` | (temp) | データディレクトリ（空=一時ディレクトリ、停止時削除） |

### libvirt (VM管理)

各ホストは個別のTCPポートで待ち受けます。go-libvirt や virsh で接続できます。

```go
// ホスト一覧を取得
resp, _ := http.Get("http://localhost:<LIBVIRT_SIM_PORT>/sim/hosts")
// => [{"host_id":"host-001","libvirt_port":17000,...}, ...]

// 各ホストにlibvirt RPCで接続
conn, _ := net.Dial("tcp", "localhost:17000") // host-001
l := libvirt.New(conn)
l.Connect()

// ドメイン定義（OVN連携用のinterfaceidを含む）
xml := `<domain type="kvm">
  <name>vm-001</name>
  <uuid>...</uuid>
  <memory unit="GiB">8</memory>
  <vcpu>4</vcpu>
  <devices>
    <disk type="network">
      <source protocol="rbd" name="cirrus-volumes/vol-001"/>
    </disk>
    <interface type="bridge">
      <source bridge="br-int"/>
      <virtualport type="openvswitch">
        <parameters interfaceid="lsp-uuid-001"/>
      </virtualport>
    </interface>
  </devices>
</domain>`
dom, _ := l.DomainDefineXMLFlags(xml, 0)
l.DomainCreate(dom)
```

対応RPCプロシージャ: ConnectOpen, ConnectClose, ConnectGetLibVersion, AuthList, DomainDefineXMLFlags, DomainLookupByUUID, DomainLookupByName, DomainGetXMLDesc, DomainGetInfo, DomainGetState, ConnectListAllDomains, DomainCreateWithFlags, DomainDestroyFlags, DomainShutdownFlags, DomainSuspend, DomainResume, DomainReboot, DomainUndefineFlags, ConnectDomainEventCallbackRegisterAny, ConnectDomainEventCallbackDeregisterAny, DomainMigratePerform3Params, DomainMigratePrepare3Params, DomainMigrateFinish3Params, DomainMigrateConfirm3Params, DomainMigrateGetMaxSpeed, DomainMigrateSetMaxSpeed

### OVN (仮想ネットワーク)

各OVNクラスタは個別のOVSDBポートで待ち受けます。標準的なOVSDB JSON-RPCプロトコルで操作します。

```bash
# Logical Switch 作成
ovn-nbctl --db=tcp:localhost:<OVN_PORT> ls-add tenant-net-001

# Logical Switch Port 作成（libvirtのinterfaceidと一致させる）
ovn-nbctl --db=tcp:localhost:<OVN_PORT> lsp-add tenant-net-001 lsp-uuid-001
ovn-nbctl --db=tcp:localhost:<OVN_PORT> lsp-set-addresses lsp-uuid-001 "fa:16:3e:aa:bb:cc 10.0.0.10"
```

対応OVSDB操作: transact (insert/select/update/delete/mutate), monitor, monitor_cancel, get_schema, list_dbs

対応テーブル (NB DB): Logical_Switch, Logical_Switch_Port, Logical_Router, Logical_Router_Port, Logical_Router_Static_Route, ACL, NAT, DHCP_Options, DNS, Load_Balancer, Address_Set, Port_Group, Gateway_Chassis

### Storage (ボリューム管理)

```bash
# ボリューム作成
curl -X POST http://localhost:<STORAGE_SIM_PORT>/api/v1/volumes \
  -H "X-Backend-Id: ceph-pool-ssd" \
  -H "Content-Type: application/json" \
  -d '{"volume_id":"vol-001","size_gb":100,"thin_provisioned":true}'

# エクスポート（ホストに接続）
curl -X POST http://localhost:<STORAGE_SIM_PORT>/api/v1/volumes/vol-001/export \
  -H "X-Backend-Id: ceph-pool-ssd" \
  -d '{"host_id":"host-001","protocol":"rbd"}'

# スナップショット
curl -X POST http://localhost:<STORAGE_SIM_PORT>/api/v1/volumes/vol-001/snapshots \
  -H "X-Backend-Id: ceph-pool-ssd" \
  -d '{"snapshot_id":"snap-001"}'
```

対応操作: volumes (CRUD, extend, export/unexport), snapshots (create/delete/clone), flatten, migration

### AWX (ジョブ実行)

```bash
# テンプレート作成
curl -X POST http://localhost:<AWX_SIM_PORT>/api/v2/job_templates/ \
  -d '{"name":"deploy-vm","expected_duration_ms":5000,"failure_rate":0.01}'

# ジョブ実行
curl -X POST http://localhost:<AWX_SIM_PORT>/api/v2/job_templates/1/launch/

# ジョブ状態確認
curl http://localhost:<AWX_SIM_PORT>/api/v2/jobs/1/

# ジョブキャンセル
curl -X POST http://localhost:<AWX_SIM_PORT>/api/v2/jobs/1/cancel/
```

### NetBox (物理トポロジ / 障害ドメイン)

NetBox v4互換API。go-netbox v4クライアントライブラリで動作確認済み。

```bash
# サイト一覧
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/sites/

# ロケーション階層（障害トポロジ: サイト → フロア → ラック列）
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/locations/
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/locations/?parent_id=0  # トップレベルのみ

# ラック（tor_switch, power_circuit付き）
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/racks/
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/racks/?site_id=1

# デバイス（cirrus_host_idでlibvirt-simのホストと紐付け）
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/devices/
curl http://localhost:<NETBOX_SIM_PORT>/api/dcim/devices/?role=server
```

レスポンスはNetBox v4形式: url, display, slug, status(value+label), nested refs, timestamps, custom_fields含む。

## VM作成の全体フロー（Cirrus想定）

Cirrusが1台のVMを作成する際の典型的なフローです:

```
1. OVN NB DB に Logical_Switch_Port を作成
   → ovn-nbctl lsp-add <switch> <lsp-uuid>

2. Storage API でボリュームを作成
   → POST /api/v1/volumes  {"volume_id":"vol-xxx","size_gb":100}

3. Storage API でボリュームをエクスポート
   → POST /api/v1/volumes/vol-xxx/export  {"host_id":"host-001"}

4. libvirt RPC でドメインを定義（interfaceid で OVN LSP と紐付け）
   → DomainDefineXMLFlags(xml_with_interfaceid_and_rbd_disk)

5. libvirt RPC でドメインを起動
   → DomainCreateWithFlags(dom)
```

シミュレータ間の直接連携は不要です。Cirrusが各シミュレータを独立に操作し、`interfaceid` と `volume_id` で論理的に紐付けます。

## 環境定義

`environments/` に環境定義YAMLがあります:

| ファイル | ホスト数 | OVNクラスタ | ストレージ | 用途 |
|----------|---------|------------|-----------|------|
| `small.yaml` | 10 | 1 | 1 | 日常開発 |
| `medium.yaml` | 400 | 2 (東京/大阪) | 2 (SSD/HDD) | 結合テスト |
| `large.yaml` | 2,500+ | 5 | 4 | 負荷テスト |

カスタム環境を使う場合:

```bash
cirrus-sim -env environments/my-env.yaml
```

シード時にNetBox物理トポロジ（サイト→ラック列→ラック→デバイス）も自動投入されます。各デバイスの`cirrus_host_id`カスタムフィールドでlibvirt-simのホストIDと対応します。

## Dashboard

Web UIダッシュボードで全シミュレータの状態を確認できます。

- 各シミュレータの統計情報（ホスト数、ドメイン数、ボリューム数等）
- ホスト/クラスタ/バックエンドの詳細ドリルダウン
- PostgreSQLテーブルブラウザ（スキーマとデータの閲覧）
- 3秒間隔の自動更新（差分更新でちらつきなし）

## 管理API

各シミュレータは `/sim/` プレフィックスで管理APIを提供します:

```bash
# ホスト一覧・追加・状態変更
GET  /sim/hosts
POST /sim/hosts  {"host_id":"host-new","libvirt_port":17099,"cpu_sockets":2,...}
PUT  /sim/hosts/{host_id}/state  {"state":"maintenance"}

# 統計
GET /sim/stats

# 全状態リセット
POST /sim/reset

# 障害注入
POST http://localhost:<COMMON_PORT>/api/v1/faults
{"target":"host-001","operation":"DomainCreate","failure_rate":0.5}

# イベントログ
GET http://localhost:<COMMON_PORT>/api/v1/events?simulator=libvirt-sim&limit=50

# PostgreSQLテーブル一覧・データ・スキーマ
GET /sim/tables
GET /sim/tables/{name}?limit=50
GET /sim/tables/{name}/schema
```

## Development

```bash
# Build all simulators
make build

# Test all
make test

# Lint
make lint

# Build & test individual simulator
make build-libvirt-sim
make test-libvirt-sim

# Build unified binary
make build-unified
./bin/cirrus-sim -version

# Build and install to /usr/local/bin
make deploy

# Run integration tests (requires running simulators)
cd tests/integration
go test -tags integration -v .
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the overall design.

## Specification

See [docs/SPEC.md](docs/SPEC.md) for the detailed simulator specification.
