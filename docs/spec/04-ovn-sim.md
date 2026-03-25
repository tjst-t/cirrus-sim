# 4. OVN シミュレータ (ovn-sim/)

## 4.1 概要

OVN Northbound DBをOVSDBプロトコル（RFC 7047準拠、JSON-RPC over TCP）で提供する。CirrusはovsdbクライアントライブラリでOVN Northbound DBと同一のスキーマに対して操作を行い、本番のovsdb-serverと完全に同じコードパスを通る。Southbound DB、ovn-northd、ovn-controllerは実装しない。

## 4.2 プロトコル

**トランスポート**: TCP（デフォルトポート: 6641 — OVN Northbound DB標準ポート）

**エンコーディング**: JSON-RPC 1.0

**接続**: `tcp:ovn-sim:6641`

マルチクラスタモード（OVN-IC用）ではクラスタごとに異なるポート:
- ovn-tokyo: 6641
- ovn-osaka: 6642
- OVN-IC Northbound: 6645

## 4.3 OVSDBプロトコルの実装

### 4.3.1 実装するRPCメソッド

| メソッド | 用途 |
|---|---|
| list_dbs | 利用可能なDB一覧（"OVN_Northbound"） |
| get_schema | OVN Northbound DBスキーマを返す |
| transact | トランザクション実行 |
| monitor | テーブル変更の購読開始 |
| monitor_cancel | 購読解除 |
| echo | ping/pong |

### 4.3.2 transactオペレーション

サポートするオペレーション:

- **insert**: 行の挿入。uuid-nameによる仮UUID参照をサポート
- **select**: 条件に基づく行の取得
- **update**: 条件に基づく行の更新
- **delete**: 条件に基づく行の削除
- **mutate**: 集合やマップの部分更新
- **wait**: 条件が満たされるまでブロック

transactリクエスト例:

```json
{
  "method": "transact",
  "params": ["OVN_Northbound",
    {
      "op": "insert",
      "table": "Logical_Switch",
      "row": {
        "name": "ls-tenant-001-net-001",
        "external_ids": ["map", [["cirrus_network_id", "net-001"]]]
      },
      "uuid-name": "new_ls"
    }
  ],
  "id": 1
}
```

### 4.3.3 monitorの実装

Cirrusがテーブルの変更を購読し、ポートのup/down等をリアルタイムに受け取る。

monitor登録:

```json
{
  "method": "monitor",
  "params": ["OVN_Northbound", "monitor-1", {
    "Logical_Switch_Port": {
      "columns": ["name", "up", "addresses", "port_security"]
    }
  }],
  "id": 2
}
```

変更があった場合、シミュレータからJSON-RPCの通知（updateメソッド）をpush送信する。

## 4.4 OVN Northbound DBスキーマ

OVNの実際のovn-nb.ovsschemaファイルを使用する。get_schemaで返し、transactのバリデーションにも使用。

主要テーブル:

| テーブル | 用途 |
|---|---|
| Logical_Switch | L2ネットワーク |
| Logical_Switch_Port | VMポート、ルータポート、localnetポート |
| Logical_Router | L3ルータ |
| Logical_Router_Port | ルータインターフェース |
| Logical_Router_Static_Route | スタティックルート |
| ACL | セキュリティグループルール |
| NAT | DNAT/SNAT |
| DHCP_Options | DHCPパラメータ |
| DNS | DNSレコード |
| Load_Balancer | ロードバランサ |
| Address_Set | IPアドレスグループ |
| Port_Group | ポートグループ |
| Gateway_Chassis | ゲートウェイ |

## 4.5 参照整合性

OVSDBスキーマのrefTableとrefTypeに基づいて参照整合性を強制する。

- **strong参照**: 参照先が存在しなければトランザクション失敗。参照先の削除時に参照元が残っていればトランザクション失敗
- **weak参照**: 参照先が削除された場合、参照が自動で除去される

重要な制約:

- Logical_Switch削除時にポートが残っていればエラー
- Logical_Router削除時にポートが残っていればエラー
- DHCP_Optionsがポートから参照されている場合、削除不可
- NATがLogical_Routerから参照されている場合、ルータ削除不可

## 4.6 OVN-ICシミュレーション

複数OVNクラスタのシミュレーション。クラスタごとにポートを分ける。IC Northbound DBも別ポートで提供。

IC Northbound DBの主要テーブル:

| テーブル | 用途 |
|---|---|
| Transit_Switch | クラスタ間接続用のトランジットスイッチ |
| Availability_Zone | 各OVNクラスタ |
| Route | クラスタ間の経路情報 |
| Port_Binding | トランジットスイッチ上のポート |

## 4.7 管理API（シミュレータ固有、REST、ポート8200）

OVSDBプロトコルとは別にシミュレータ管理用REST APIを提供する。

```
POST   /sim/clusters                 OVNクラスタ登録
GET    /sim/clusters                 クラスタ一覧
POST   /sim/ic/connections           IC接続の設定
GET    /sim/stats                    全体統計
POST   /sim/reset                    全状態リセット
POST   /sim/ports/{port_uuid}/up     ポートのup状態設定（VM起動連動テスト用）
POST   /sim/ports/{port_uuid}/down   ポートのdown状態設定
```
