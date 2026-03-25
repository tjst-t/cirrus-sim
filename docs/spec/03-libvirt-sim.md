# 3. libvirt シミュレータ (libvirt-sim/)

## 3.1 概要

複数の仮想ホストをシミュレーションし、libvirt RPCプロトコル（XDR over TCP）で通信する。QEMUプロセスは一切起動せず、全てインメモリの状態管理で動作する。Cirrusは `go-libvirt` や `libvirt-go-module` で接続し、本番のlibvirtdと完全に同じコードパスを通る。

## 3.2 プロトコル

**トランスポート**: TCP（デフォルトポート: 16509）

**エンコーディング**: XDR (RFC 4506) によるlibvirt RPCメッセージ

**接続URI**: `qemu+tcp://libvirt-sim:16509/system`

マルチホストモードでは、ホストごとに異なるTCPポートを割り当てる（host-001→16510, host-002→16511, ...）。数千ホストの場合はポート範囲を設定で指定。

## 3.3 実装するRPCプロシージャ

libvirtは300以上のRPCプロシージャを持つが、Cirrusが使用するサブセットのみ実装する。

### 3.3.1 接続管理

| プロシージャ | 用途 |
|---|---|
| REMOTE_PROC_CONNECT_OPEN | 接続確立 |
| REMOTE_PROC_CONNECT_CLOSE | 接続切断 |
| REMOTE_PROC_CONNECT_GET_HOSTNAME | ホスト名取得 |
| REMOTE_PROC_CONNECT_GET_VERSION | libvirtバージョン取得 |
| REMOTE_PROC_CONNECT_GET_CAPABILITIES | ホストcapability XML取得 |

### 3.3.2 ホスト情報

| プロシージャ | 用途 |
|---|---|
| REMOTE_PROC_NODE_GET_INFO | CPU/メモリ情報 |
| REMOTE_PROC_NODE_GET_CPU_STATS | CPU使用率 |
| REMOTE_PROC_NODE_GET_MEMORY_STATS | メモリ使用率 |
| REMOTE_PROC_NODE_GET_FREE_MEMORY | 空きメモリ |
| REMOTE_PROC_CONNECT_GET_ALL_DOMAIN_STATS | 全ドメイン統計 |

### 3.3.3 ドメイン管理

| プロシージャ | 用途 |
|---|---|
| REMOTE_PROC_DOMAIN_DEFINE_XML_FLAGS | domain XMLからVM定義 |
| REMOTE_PROC_DOMAIN_CREATE_WITH_FLAGS | VM起動 |
| REMOTE_PROC_DOMAIN_DESTROY_FLAGS | VM強制停止 |
| REMOTE_PROC_DOMAIN_SHUTDOWN_FLAGS | VMグレースフル停止 |
| REMOTE_PROC_DOMAIN_REBOOT | VM再起動 |
| REMOTE_PROC_DOMAIN_SUSPEND | VM一時停止 |
| REMOTE_PROC_DOMAIN_RESUME | VM再開 |
| REMOTE_PROC_DOMAIN_UNDEFINE_FLAGS | VM定義削除 |
| REMOTE_PROC_DOMAIN_GET_XML_DESC | domain XML取得 |
| REMOTE_PROC_DOMAIN_GET_INFO | ドメイン基本情報 |
| REMOTE_PROC_DOMAIN_GET_STATE | ドメイン状態 |
| REMOTE_PROC_DOMAIN_LIST_ALL_DOMAINS | ドメイン一覧 |
| REMOTE_PROC_DOMAIN_LOOKUP_BY_UUID | UUID検索 |
| REMOTE_PROC_DOMAIN_LOOKUP_BY_NAME | 名前検索 |

### 3.3.4 ライブマイグレーション

| プロシージャ | 用途 |
|---|---|
| REMOTE_PROC_DOMAIN_MIGRATE_PERFORM3_PARAMS | マイグレーション実行（移行元） |
| REMOTE_PROC_DOMAIN_MIGRATE_PREPARE3_PARAMS | マイグレーション準備（移行先） |
| REMOTE_PROC_DOMAIN_MIGRATE_CONFIRM3_PARAMS | マイグレーション確認（移行元） |
| REMOTE_PROC_DOMAIN_MIGRATE_FINISH3_PARAMS | マイグレーション完了（移行先） |
| REMOTE_PROC_DOMAIN_MIGRATE_GET_MAX_SPEED | 最大帯域取得 |
| REMOTE_PROC_DOMAIN_MIGRATE_SET_MAX_SPEED | 最大帯域設定 |

### 3.3.5 イベント

| プロシージャ | 用途 |
|---|---|
| REMOTE_PROC_CONNECT_DOMAIN_EVENT_REGISTER_ANY | ドメインイベント購読 |
| REMOTE_PROC_CONNECT_DOMAIN_EVENT_DEREGISTER_ANY | 購読解除 |
| REMOTE_PROC_DOMAIN_EVENT_LIFECYCLE | ライフサイクルイベント通知 |

イベント通知はCirrusがVM状態変化をリアルタイムに検知するために重要。シミュレータは状態遷移時にイベントをpush通知する。

## 3.4 内部状態モデル

### 3.4.1 ドメイン状態遷移

```
              start
  shutoff ───────────→ running
    ↑                    │  │
    │  shutdown/destroy  │  │ suspend
    │←───────────────────┘  │
    │                       ↓
    │                    paused
    │                      │
    │     resume           │
    │←── running ←─────────┘
```

不正な状態遷移に対してはlibvirtと同一のエラーコードを返す:

- VIR_ERR_OPERATION_INVALID: 現在の状態では許可されない操作
- VIR_ERR_NO_DOMAIN: 存在しないドメイン
- VIR_ERR_OPERATION_DENIED: リソース不足

### 3.4.2 リソース消費追跡

```
used_vcpus     = Σ(running domainのvcpus)
used_memory_mb = Σ(running domainのmemory_mb)
used_gpus      = running domainにpassthroughされたGPUの集合
```

overcommit_ratioをホストごとに設定可能。割当可能量を超えるドメインのstartはVIR_ERR_OPERATION_DENIEDを返す。

### 3.4.3 NUMAリソース追跡

NUMAプレースメントが指定されたドメインは指定ノードのリソースを消費する。ノードの割当可能量を超えた場合はエラー。GPUパススルーは常に排他割当。

## 3.5 ライブマイグレーション

libvirtのライブマイグレーションはマルチフェーズプロトコル。シミュレータは2つのホスト（ポート）間でこのプロトコルを正確に再現する。

### 3.5.1 フェーズ

1. **Prepare (移行先)**: 移行先ホストでリソースを予約
2. **Perform (移行元)**: メモリのイテレーティブコピーをシミュレーション
3. **Finish (移行先)**: ドメインを移行先で起動
4. **Confirm (移行元)**: 移行元のドメインを除去

### 3.5.2 シミュレーション設定

管理API経由で設定:

```json
{
  "prepare_duration_ms": 500,
  "base_transfer_duration_ms": 2000,
  "per_gb_memory_ms": 500,
  "finish_duration_ms": 200
}
```

transfer_durationはドメインのメモリサイズに比例:
`transfer_duration = base_transfer_duration_ms + (memory_gb * per_gb_memory_ms)`

### 3.5.3 前提条件チェック

- ドメインがrunning状態であること
- 移行先にリソースの空きがあること（CPU、メモリ、GPU、NUMAノード）
- 移行先ホストがrunning状態であること
- 同一ドメインの既存マイグレーションが進行中でないこと

不満足ならlibvirtの適切なエラーコードを返す。

## 3.6 Domain XMLの処理

CirrusはVM定義をlibvirt domain XMLとして構築しlibvirtdに送る。シミュレータはXMLをパースして内部状態に変換する。完全なXMLスキーマバリデーションは不要だが、Cirrusが使用するフィールドは正確にパース・保持・返却する。

対応するXMLセクション:

- `<vcpu>`, `<memory>`: リソース量
- `<cpu>`: CPUモデル、NUMAトポロジ
- `<numatune>`: NUMAプレースメント
- `<devices><disk>`: ディスク定義（volume IDの抽出）
- `<devices><interface>`: ネットワークインターフェース（OVNポートIDの抽出）
- `<devices><hostdev>`: GPUパススルー
- `<metadata>`: Cirrus固有のメタデータ

## 3.7 管理API（シミュレータ固有、REST、ポート8100）

libvirt RPCプロトコルとは別に、シミュレータの管理用REST APIを提供する。ホストの登録、設定変更など、本番のlibvirtdには存在しないシミュレータ固有の操作に使用する。

```
POST   /sim/hosts                    ホスト登録
GET    /sim/hosts                    ホスト一覧
GET    /sim/hosts/{host_id}          ホスト情報（リソース使用状況含む）
PUT    /sim/hosts/{host_id}/state    ホスト状態変更（maintenance, error等）
POST   /sim/hosts/{host_id}/config   オーバーコミット率等の設定
GET    /sim/stats                    全体統計
POST   /sim/reset                    全状態リセット
POST   /sim/config/migration         マイグレーション設定
```

ホスト登録リクエスト:

```json
{
  "host_id": "host-042",
  "libvirt_port": 16510,
  "cpu_model": "Intel Xeon Gold 6348",
  "cpu_sockets": 2,
  "cores_per_socket": 28,
  "threads_per_core": 2,
  "memory_mb": 524288,
  "cpu_overcommit_ratio": 4.0,
  "memory_overcommit_ratio": 1.5,
  "numa_topology": [
    {
      "node_id": 0,
      "cpus": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
               56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69],
      "memory_mb": 262144,
      "gpus": ["gpu-0", "gpu-1"]
    },
    {
      "node_id": 1,
      "cpus": [28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41,
               84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97],
      "memory_mb": 262144,
      "gpus": ["gpu-2", "gpu-3"]
    }
  ],
  "gpus": [
    {"id": "gpu-0", "model": "NVIDIA A100", "memory_mb": 81920},
    {"id": "gpu-1", "model": "NVIDIA A100", "memory_mb": 81920},
    {"id": "gpu-2", "model": "NVIDIA A100", "memory_mb": 81920},
    {"id": "gpu-3", "model": "NVIDIA A100", "memory_mb": 81920}
  ]
}
```
