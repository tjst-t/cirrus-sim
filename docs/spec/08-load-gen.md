# 8. 負荷ジェネレータ (load-gen/)

## 8.1 概要

Cirrusに対して大量のAPIリクエストを発行し、スケーラビリティとパフォーマンスを検証する。

## 8.2 ワークロード定義

```yaml
workload:
  name: "vm-creation-burst"
  target: "http://cirrus:8080/api/v1"

  phases:
    - name: "ramp-up"
      duration_sec: 30
      actions:
        - type: "create_vm"
          rate_per_sec: 5
          params:
            vcpus: [2, 4, 8]
            memory_mb: [4096, 8192, 16384]
            volume_type: "ssd"
            volume_size_gb: [50, 100, 200]
            network: "auto"

    - name: "steady-state"
      duration_sec: 300
      actions:
        - type: "create_vm"
          rate_per_sec: 10
        - type: "delete_vm"
          rate_per_sec: 8
        - type: "live_migrate_vm"
          rate_per_sec: 1

    - name: "chaos"
      duration_sec: 60
      actions:
        - type: "host_failure"
          rate_per_sec: 0.1
        - type: "create_vm"
          rate_per_sec: 5

  assertions:
    - metric: "api_response_time_p99"
      threshold_ms: 500
    - metric: "vm_creation_success_rate"
      threshold_percent: 99
    - metric: "migration_success_rate"
      threshold_percent: 95
    - metric: "scheduler_decision_time_p99"
      threshold_ms: 100
```

## 8.3 アクションタイプ

| アクション | 説明 |
|-----------|------|
| create_vm | VM作成（ボリューム・ネットワーク含む） |
| delete_vm | ランダムなVMを削除 |
| live_migrate_vm | ランダムなVMをライブマイグレーション |
| create_snapshot | ランダムなボリュームのスナップショット作成 |
| create_volume | 追加ボリューム作成 |
| host_failure | ランダムなホストを障害状態にする |
| storage_drain | ランダムなバックエンドをドレイン開始 |

## 8.4 実行・結果取得

**実行**:

```
POST /api/v1/workloads/run
Content-Type: application/yaml
(ワークロードYAML本文)
```

**結果取得**:

```
GET /api/v1/workloads/run/{run_id}
```

レスポンス:

```json
{
  "run_id": "run-001",
  "status": "completed",
  "results": {
    "total_requests": 5200,
    "successful_requests": 5180,
    "failed_requests": 20,
    "api_response_time_p50_ms": 45,
    "api_response_time_p99_ms": 320,
    "scheduler_decision_time_p50_ms": 12,
    "scheduler_decision_time_p99_ms": 78,
    "vm_creation_success_rate": 99.6,
    "migration_success_rate": 96.2
  },
  "assertions": {
    "api_response_time_p99": {"threshold_ms": 500, "actual_ms": 320, "passed": true},
    "vm_creation_success_rate": {"threshold_percent": 99, "actual_percent": 99.6, "passed": true},
    "migration_success_rate": {"threshold_percent": 95, "actual_percent": 96.2, "passed": true},
    "scheduler_decision_time_p99": {"threshold_ms": 100, "actual_ms": 78, "passed": true}
  }
}
```
