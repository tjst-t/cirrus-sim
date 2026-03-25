# 6. AWX シミュレータ (awx-sim/)

## 6.1 概要

AWX/Ansible Tower のREST APIの最小サブセットを提供する。

ベースURL: `http://awx-sim:8300/api/v2`

## 6.2 ジョブテンプレートAPI

**登録**: `POST /job_templates/`

```json
{
  "id": 1,
  "name": "host-provision",
  "description": "Provision a new host with base OS",
  "expected_duration_ms": 30000,
  "failure_rate": 0.0
}
```

`expected_duration_ms` と `failure_rate` はシミュレータ固有フィールド。

**一覧**: `GET /job_templates/`

## 6.3 ジョブ実行API

**起動**: `POST /job_templates/{id}/launch/`

```json
{
  "extra_vars": {
    "host_id": "host-042",
    "profile_version": "v2",
    "target_kernel": "5.15.0-100"
  }
}
```

**状態取得**: `GET /jobs/{id}/`

**状態遷移**: pending → running → successful/failed (または canceled)

遷移タイミング:
- pending → running: 即座（設定で遅延可能）
- running → successful/failed: `expected_duration_ms` 後に `failure_rate` で判定

**キャンセル**: `POST /jobs/{id}/cancel/`

pending または running 状態のみ。

## 6.4 コールバック

ジョブ完了時にCirrusの指定URLにPOSTを送信する。

```
POST /sim/config/callback
```

```json
{
  "enabled": true,
  "callback_url": "http://cirrus:8080/api/v1/hooks/callback",
  "auth_token": "cirrus-callback-token"
}
```

コールバックのペイロード:

```json
{
  "job_id": 101,
  "job_template_id": 1,
  "status": "successful",
  "extra_vars": {},
  "finished": "2025-01-01T10:00:31Z"
}
```

## 6.5 管理API

```
GET    /sim/stats         ジョブ統計（実行中、成功、失敗の数）
POST   /sim/reset         全状態リセット
```
