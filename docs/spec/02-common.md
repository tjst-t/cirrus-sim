# 2. 共通ライブラリ (common/)

## 2.1 障害注入エンジン (fault_injection/)

全シミュレータから利用される障害注入の共通フレームワーク。

### 2.1.1 障害の種類

```yaml
fault_types:
  error:
    description: "API呼び出しがエラーを返す"
    parameters:
      - error_code: int
      - error_message: string

  delay:
    description: "API呼び出しの応答が遅延する"
    parameters:
      - min_ms: int
      - max_ms: int

  timeout:
    description: "API呼び出しが応答しない（タイムアウト）"
    parameters:
      - after_ms: int

  partial_failure:
    description: "操作の途中で失敗する"
    parameters:
      - fail_at_percent: int
```

### 2.1.2 障害の適用条件

```yaml
fault_rule:
  target:
    simulator: "libvirt-sim"
    host_id: "host-042"
    operation: "migrate"

  trigger:
    type: "probabilistic"
    probability: 0.3

    # または
    type: "count"
    after_count: 5
    repeat: false

    # または
    type: "time"
    activate_at: "2025-01-01T10:00:00Z"
    duration_sec: 300

  fault:
    type: "error"
    error_code: -1
    error_message: "Connection refused"
```

### 2.1.3 障害注入API

```
POST   /api/v1/faults           障害ルールを追加
GET    /api/v1/faults           全障害ルールを取得
DELETE /api/v1/faults/{id}      障害ルールを削除
DELETE /api/v1/faults           全障害ルールをクリア
```

## 2.2 イベントログ (event_log/)

全シミュレータの操作を時系列で記録する。

### 2.2.1 イベントの構造

```json
{
  "timestamp": "2025-01-01T10:00:00.123Z",
  "simulator": "libvirt-sim",
  "host_id": "host-042",
  "operation": "domain_create",
  "request_summary": "vm-001, 4vcpu, 8192MB",
  "result": "success",
  "duration_ms": 12,
  "fault_injected": false
}
```

### 2.2.2 イベントログAPI

```
GET    /api/v1/events                          全イベント取得（ページネーション付き）
GET    /api/v1/events?simulator=libvirt-sim     シミュレータでフィルタ
GET    /api/v1/events?host_id=host-042          ホストでフィルタ
GET    /api/v1/events?after=<timestamp>         時刻でフィルタ
DELETE /api/v1/events                           全イベントをクリア
```

## 2.3 状態管理

全シミュレータの状態をスナップショット・リストアする。

```
POST   /api/v1/state/snapshot          全シミュレータの状態を保存 → snapshot_id返却
POST   /api/v1/state/restore/{id}      指定スナップショットに復元
GET    /api/v1/state/snapshots          スナップショット一覧
DELETE /api/v1/state/snapshots/{id}     スナップショット削除
```

## 2.4 データジェネレータ (data_generator/)

環境定義YAMLから大規模環境を一発生成する。

### 2.4.1 環境定義の構造

```yaml
environment:
  name: "large-scale-test"

  sites:
    - name: "site-tokyo"
      rack_rows: 5
      racks_per_row: 10
      hosts_per_rack: 6
      host_template: "standard-compute"

    - name: "site-osaka"
      rack_rows: 3
      racks_per_row: 8
      hosts_per_rack: 6
      host_template: "standard-compute"

  host_templates:
    standard-compute:
      cpu_model: "Intel Xeon Gold 6348"
      cpu_sockets: 2
      cores_per_socket: 28
      threads_per_core: 2
      memory_gb: 512
      numa_nodes: 2
      nics:
        - name: "ens1f0"
          speed_gbps: 25
          sriov: true
        - name: "ens2f0"
          speed_gbps: 25
          sriov: false
      local_disks:
        - type: "nvme"
          size_gb: 1600

    gpu-compute:
      inherits: "standard-compute"
      memory_gb: 1024
      gpus:
        - model: "NVIDIA A100"
          count: 4
          numa_distribution: [2, 2]

  storage_backends:
    - name: "ceph-pool-ssd"
      type: "ceph"
      total_capacity_tb: 500
      total_iops: 500000
      capabilities: ["ssd", "snapshot", "clone", "live_migration", "differential_transfer"]
      accessible_from: ["site-tokyo"]

    - name: "ceph-pool-hdd"
      type: "ceph"
      total_capacity_tb: 5000
      total_iops: 50000
      capabilities: ["hdd", "snapshot", "clone"]
      accessible_from: ["site-tokyo", "site-osaka"]

  ovn_clusters:
    - name: "ovn-tokyo"
      covers: ["site-tokyo"]
    - name: "ovn-osaka"
      covers: ["site-osaka"]

    ovn_ic:
      enabled: true
      connects: ["ovn-tokyo", "ovn-osaka"]

  preload:
    vms_per_host:
      min: 30
      max: 70
    volumes_per_vm:
      min: 1
      max: 3
    networks_per_tenant: 2
    tenants: 50
```

### 2.4.2 データジェネレータAPI

```
POST   /api/v1/generate          環境定義YAMLを投入して環境を生成
GET    /api/v1/generate/status    生成の進捗を取得
POST   /api/v1/generate/reset     全データをクリア
```
