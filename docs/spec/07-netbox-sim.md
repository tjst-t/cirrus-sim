# 7. NetBox シミュレータ (netbox-sim/)

## 7.1 概要

NetBoxのREST APIの最小サブセットを提供する。障害トポロジツリーとホストの物理位置情報をCirrusに提供する。

ベースURL: `http://netbox-sim:8400/api`

## 7.2 サイトAPI

```
GET /dcim/sites/
```

レスポンス:

```json
{
  "results": [
    {
      "id": 1,
      "name": "site-tokyo",
      "status": {"value": "active"},
      "region": {"id": 1, "name": "japan"},
      "custom_fields": {
        "power_feed_group": "pfg-tokyo-main",
        "upstream_switch": "spine-tokyo-01"
      }
    }
  ]
}
```

## 7.3 ラックAPI

```
GET /dcim/racks/
GET /dcim/racks/?site_id=1
```

レスポンス:

```json
{
  "results": [
    {
      "id": 1,
      "name": "rack-A01",
      "site": {"id": 1, "name": "site-tokyo"},
      "location": {"id": 1, "name": "row-A"},
      "status": {"value": "active"},
      "custom_fields": {
        "power_circuit": "pdu-a01-1",
        "tor_switch": "tor-a01"
      }
    }
  ]
}
```

## 7.4 デバイス（ホスト）API

```
GET /dcim/devices/
GET /dcim/devices/?rack_id=1
GET /dcim/devices/?role=server
```

レスポンス:

```json
{
  "results": [
    {
      "id": 1,
      "name": "host-042",
      "device_role": {"name": "server"},
      "site": {"id": 1, "name": "site-tokyo"},
      "rack": {"id": 1, "name": "rack-A01"},
      "position": 20,
      "status": {"value": "active"},
      "custom_fields": {
        "cirrus_host_id": "host-042"
      }
    }
  ]
}
```

## 7.5 障害トポロジの導出

Cirrusの同期アダプタは以下の階層を構築する:

```
site (power_feed_group, upstream_switch)
  └─ location/row
       └─ rack (power_circuit, tor_switch)
            └─ device (unit position)
```

同一のpower_circuit、tor_switch等を持つデバイス群が障害共有グループとなる。

## 7.6 データ投入API（シミュレータ固有）

```
POST /sim/bulk-load
```

```json
{
  "sites": [
    {
      "name": "site-tokyo",
      "locations": [
        {
          "name": "row-A",
          "racks": [
            {
              "name": "rack-A01",
              "tor_switch": "tor-a01",
              "power_circuit": "pdu-a01-1",
              "devices": [
                {"name": "host-001", "position": 40, "cirrus_host_id": "host-001"},
                {"name": "host-002", "position": 38, "cirrus_host_id": "host-002"}
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

## 7.7 管理API

```
GET    /sim/stats     統計（サイト数、ラック数、デバイス数）
POST   /sim/reset     全状態リセット
```
