# シミュレータ間整合性チェック

## 概要

Cirrusが各シミュレータを独立に操作するため、操作の順序やエラーによってシミュレータ間の状態に不整合が生じる場合がある。整合性チェッカーは、全シミュレータのAPIを外部から読み取って突合し、不整合を検出する。

シミュレータ間の直接連携は行わない（設計原則）。チェッカーは読み取り専用で、不整合を報告するのみ。自動修復は行わない。

## 不整合レベル

| レベル | 意味 | 例 |
|--------|------|-----|
| **error** | 明らかに壊れている。存在しないリソースへの参照 | VMが存在しないボリュームを参照 |
| **warning** | 片方にしかない。意図的な場合もあるが確認推奨 | OVNにLSPがあるがどのVMにも使われていない |
| **info** | 参考情報。正常な途中状態の可能性が高い | ボリュームがエクスポートされているがVMにまだ未割当 |

## チェック項目

### libvirt-sim ↔ storage-sim

#### E-001: VMが存在しないボリュームを参照

- **レベル**: error
- **条件**: ドメインXMLの `<disk>` の `<source name="..."/>` で参照されるボリュームIDがstorage-simに存在しない
- **データ取得**:
  - libvirt-sim: `GET /sim/hosts` → 各ホストのドメインXMLからdisk sourceを抽出
  - storage-sim: `GET /api/v1/volumes`（全バックエンド）
- **出力**: VM名、ホストID、参照ボリュームID

#### E-002: VMが参照するボリュームがホストにエクスポートされていない

- **レベル**: error
- **条件**: ドメインXMLがボリュームを参照し、そのVMが配置されているホストにそのボリュームがエクスポートされていない
- **データ取得**:
  - libvirt-sim: ドメインXMLからdisk source + ホストID
  - storage-sim: `GET /api/v1/volumes/{id}` のexport_info
- **出力**: VM名、ホストID、ボリュームID、エクスポート先ホスト（またはnull）

#### E-003: 存在しないホストへのボリュームエクスポート

- **レベル**: error
- **条件**: storage-simでボリュームのexport_infoのhost_idがlibvirt-simのホスト一覧に存在しない
- **データ取得**:
  - storage-sim: 全ボリュームのexport_info
  - libvirt-sim: `GET /sim/hosts`
- **出力**: ボリュームID、バックエンドID、エクスポート先ホストID

### libvirt-sim ↔ ovn-sim

#### E-004: VMが存在しないOVN LSPを参照

- **レベル**: error
- **条件**: ドメインXMLの `<virtualport>` の `interfaceid` に対応するLogical_Switch_PortがOVN NB DBに存在しない
- **データ取得**:
  - libvirt-sim: ドメインXMLからinterfaceidを抽出
  - ovn-sim: OVSDBのLogical_Switch_Portテーブルを全件取得
- **出力**: VM名、ホストID、interfaceid

#### W-001: 孤立OVN LSP

- **レベル**: warning
- **条件**: OVN NB DBにLogical_Switch_Portが存在するが、どのVMのinterfaceidにも使われていない
- **補足**: テナントネットワーク作成後にVM作成前の途中状態では正常。長期間残っている場合はリーク
- **データ取得**: E-004と同じデータの逆方向チェック
- **出力**: LSP名、所属Logical Switch

#### E-005: running VMのinterfaceidにLSPがない

- **レベル**: error
- **条件**: E-004のサブセット。VMがrunning状態の場合のみ。shutoff VMは定義だけで未接続の場合がある
- **データ取得**: E-004と同じ + ドメインの状態フィルタ
- **出力**: VM名、ホストID、interfaceid

### libvirt-sim ↔ netbox-sim

#### W-002: libvirtホストがNetBoxに登録されていない

- **レベル**: warning
- **条件**: libvirt-simにホストが存在するが、netbox-simのdeviceにcirrus_host_id=<host_id>が見つからない
- **補足**: シーディング時は自動で作られるが、動的にホストを追加した場合に発生しうる
- **データ取得**:
  - libvirt-sim: `GET /sim/hosts`
  - netbox-sim: `GET /api/dcim/devices/?role=server`
- **出力**: ホストID

#### W-003: NetBoxデバイスに対応するlibvirtホストがない

- **レベル**: warning
- **条件**: netbox-simにcirrus_host_id付きのdeviceがあるが、libvirt-simに対応するホストがない
- **データ取得**: W-002と同じデータの逆方向チェック
- **出力**: NetBoxデバイス名、cirrus_host_id

### storage-sim 内部

#### E-006: 容量超過

- **レベル**: error
- **条件**: バックエンドのallocated_capacity_gbがtotal_capacity_gb × overprovision_ratioを超えている
- **データ取得**: `GET /sim/backends`
- **出力**: バックエンドID、allocated、limit

## APIレスポンス形式

```
GET /sim/consistency-check
```

```json
{
  "status": "inconsistent",
  "summary": {
    "error": 1,
    "warning": 2,
    "info": 0
  },
  "checks": [
    {
      "id": "E-002",
      "name": "vm_disk_exported",
      "level": "error",
      "status": "fail",
      "message": "1 VM references a volume not exported to its host",
      "details": [
        {
          "vm": "vm-001",
          "host": "host-001",
          "volume": "vol-003",
          "issue": "volume not exported to host-001"
        }
      ]
    },
    {
      "id": "W-001",
      "name": "orphan_lsp",
      "level": "warning",
      "status": "fail",
      "message": "2 LSPs are not referenced by any VM",
      "details": [
        {"lsp": "lsp-orphan-001", "switch": "tenant-net-001"},
        {"lsp": "lsp-orphan-002", "switch": "tenant-net-002"}
      ]
    },
    {
      "id": "E-001",
      "name": "vm_disk_exists",
      "level": "error",
      "status": "ok",
      "message": "all VM disk references are valid"
    }
  ]
}
```

## 実装方針

- チェッカーはシミュレータの外部に配置（dashboard or common）
- 各シミュレータの既存REST APIを読み取り専用で呼び出す
- シミュレータのコードには手を入れない
- CLIからも呼び出せる（`cirrus-sim-ctl check`）
- ダッシュボードにステータスを表示（error/warningの数）
