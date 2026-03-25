# 5. ストレージシミュレータ (storage-sim/)

## 5.1 概要

Cirrus Storage APIを実装するインメモリのストレージバックエンドシミュレータ。本番ではバックエンドごとのstorage-agent（Ceph agent、NFS agent等）がこのAPIを実装し、内部でlibrbd/NFS操作に変換する。storage-simは実データを一切持たず、メタデータと容量の追跡のみ行う。

## 5.2 プロトコル

**トランスポート**: HTTP/REST

**ベースURL**: `http://storage-sim:8500/api/v1`

マルチバックエンドモード: 1プロセスで複数バックエンドをシミュレーション。リクエストヘッダ `X-Backend-Id` でバックエンドを指定。

## 5.3 Cirrus Storage API

本番のstorage-agentとstorage-simの両方が実装する共通APIインターフェース。このAPIがCirrusとストレージ間の契約となる。

### 5.3.1 バックエンド情報

```
GET /backend/info
```

レスポンス:

```json
{
  "backend_id": "ceph-pool-ssd",
  "total_capacity_gb": 512000,
  "used_capacity_gb": 184320,
  "allocated_capacity_gb": 307200,
  "total_iops": 500000,
  "used_iops_estimate": 120000,
  "capabilities": ["ssd", "snapshot", "clone", "live_migration", "differential_transfer", "qos"],
  "state": "active"
}
```

```
GET /backend/health
```

レスポンス:

```json
{
  "healthy": true,
  "latency_ms": 2,
  "last_check": "2025-01-01T10:00:00Z"
}
```

### 5.3.2 ボリュームCRUD

**作成**: `POST /volumes`

```json
{
  "volume_id": "vol-001",
  "size_gb": 100,
  "thin_provisioned": true,
  "qos_policy": {
    "max_iops": 5000,
    "max_bandwidth_mbps": 200
  },
  "metadata": {
    "cirrus_tenant_id": "tenant-001",
    "cirrus_vm_id": "vm-001"
  }
}
```

レスポンス:

```json
{
  "volume_id": "vol-001",
  "size_gb": 100,
  "consumed_gb": 0,
  "state": "available",
  "thin_provisioned": true,
  "parent_snapshot_id": "",
  "qos_policy": {"max_iops": 5000, "max_bandwidth_mbps": 200},
  "created_at": "2025-01-01T10:00:00Z"
}
```

**取得**: `GET /volumes/{volume_id}`

**一覧**: `GET /volumes` , `GET /volumes?state=in_use`

**削除**: `DELETE /volumes/{volume_id}`

前提条件: stateがavailable（in_useなら406）、スナップショットなし（あれば409）

**リサイズ**: `PUT /volumes/{volume_id}/extend`

```json
{"new_size_gb": 200}
```

縮小は不可（400エラー）。

### 5.3.3 エクスポート/アンエクスポート

VMにアタッチするためのエクスポート情報を取得。

**エクスポート**: `POST /volumes/{volume_id}/export`

```json
{
  "host_id": "host-042",
  "protocol": "rbd"
}
```

レスポンス:

```json
{
  "export_type": "rbd",
  "connection_info": {
    "pool": "cirrus-volumes",
    "image": "vol-001",
    "monitors": ["10.0.1.1:6789", "10.0.1.2:6789", "10.0.1.3:6789"],
    "auth": {
      "username": "cirrus",
      "key_ring": "/etc/ceph/cirrus.keyring"
    }
  }
}
```

ボリュームstate: available → in_use

**アンエクスポート**: `DELETE /volumes/{volume_id}/export`

ボリュームstate: in_use → available

### 5.3.4 スナップショット

**作成**: `POST /volumes/{volume_id}/snapshots`

```json
{
  "snapshot_id": "snap-001",
  "metadata": {"description": "before-upgrade"}
}
```

レスポンス:

```json
{
  "snapshot_id": "snap-001",
  "volume_id": "vol-001",
  "size_gb": 100,
  "consumed_gb": 0,
  "state": "available",
  "child_clones": [],
  "created_at": "2025-01-01T10:00:00Z"
}
```

**一覧**: `GET /volumes/{volume_id}/snapshots`

**削除**: `DELETE /snapshots/{snapshot_id}`

前提条件: child_clonesが空（空でなければ409）

### 5.3.5 クローン

```
POST /snapshots/{snapshot_id}/clone
```

```json
{
  "volume_id": "vol-002",
  "metadata": {"cirrus_tenant_id": "tenant-001"}
}
```

レスポンス: ボリュームオブジェクト（parent_snapshot_idがセット）。元スナップショットのchild_clonesにvol-002が追加。

### 5.3.6 フラット化

クローンの親依存を切る。非同期操作。

`POST /volumes/{volume_id}/flatten`

レスポンス:

```json
{
  "operation_id": "op-001",
  "state": "running",
  "progress_percent": 0,
  "estimated_duration_ms": 60000
}
```

`GET /operations/{operation_id}`

レスポンス:

```json
{
  "operation_id": "op-001",
  "type": "flatten",
  "volume_id": "vol-002",
  "state": "running",
  "progress_percent": 45,
  "elapsed_ms": 27000,
  "estimated_remaining_ms": 33000
}
```

完了時: consumed_gb = size_gb、parent_snapshot_id = ""、元スナップショットのchild_clonesからvolumeを除去。

### 5.3.7 ストレージマイグレーション

非同期操作。

**開始**: `POST /migrations`

```json
{
  "volume_id": "vol-001",
  "dest_backend_url": "http://storage-sim:8500/api/v1",
  "dest_backend_id": "ceph-pool-hdd"
}
```

前提条件: スナップショットなし（あれば先にflatten、409エラー）、移行先に空き容量あり

**状態取得**: `GET /migrations/{migration_id}`

```json
{
  "migration_id": "mig-001",
  "volume_id": "vol-001",
  "source_backend_id": "ceph-pool-ssd",
  "dest_backend_id": "ceph-pool-hdd",
  "state": "transferring",
  "progress_percent": 60,
  "bytes_transferred": 64424509440,
  "elapsed_ms": 120000,
  "estimated_remaining_ms": 80000
}
```

状態遷移: preparing → transferring → finishing → completed (または failed)

**キャンセル**: `DELETE /migrations/{migration_id}`

完了時: 移行元から除去、移行先に登録、stateが元に戻る。

## 5.4 シミュレーション動作

### 5.4.1 容量追跡

```
used_capacity_gb      = Σ(volume.consumed_gb) + Σ(snapshot.consumed_gb)
allocated_capacity_gb = Σ(volume.size_gb)

ボリューム作成時:
  thin_provisioned:
    allocated_capacity_gb += volume.size_gb
    チェック: allocated_capacity_gb <= total_capacity_gb * overprovision_ratio
  非thin_provisioned:
    used_capacity_gb += volume.size_gb
    チェック: used_capacity_gb <= total_capacity_gb
```

### 5.4.2 シンプロビジョニングシミュレーション

consumed_gbの時間経過による増加（テスト用、設定可能）。

### 5.4.3 非同期操作の時間設定

```json
{
  "timing": {
    "snapshot_create_ms": 500,
    "clone_create_ms": 1000,
    "flatten_per_gb_ms": 100,
    "migration_per_gb_ms": 200
  }
}
```

## 5.5 管理API（シミュレータ固有、/sim/ プレフィックス）

```
POST   /sim/backends                  バックエンド登録
GET    /sim/backends                  バックエンド一覧
PUT    /sim/backends/{id}/state       バックエンド状態変更（active, draining, read_only, retired）
POST   /sim/config                    シミュレーション設定（タイミング、シンプロビジョニング等）
GET    /sim/stats                     全体統計
POST   /sim/reset                     全状態リセット
```

バックエンド登録:

```json
{
  "backend_id": "ceph-pool-ssd",
  "total_capacity_gb": 512000,
  "total_iops": 500000,
  "capabilities": ["ssd", "snapshot", "clone", "live_migration", "differential_transfer", "qos"],
  "overprovision_ratio": 2.0
}
```
