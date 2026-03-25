# Cirrus-Sim アーキテクチャ

## 全体構成

```
Cirrus (IaaS本体)
  │
  ├── libvirt RPC ──→ libvirtd (本番) / libvirt-sim (開発)
  ├── OVSDB ────────→ ovsdb-server (本番) / ovn-sim (開発)
  ├── Storage API ──→ storage-agent (本番) / storage-sim (開発)
  ├── AWX REST ─────→ AWX (本番) / awx-sim (開発)
  └── NetBox REST ──→ NetBox (本番) / netbox-sim (開発)
```

Cirrus本体は接続先エンドポイントの設定変更のみで本番とシミュレータを切り替える。
シミュレータ側でプロトコルを変える（例: libvirt RPCをRESTに差し替える）ことは禁止。

## プロトコル詳細

### libvirt-sim

- **プロトコル**: libvirt RPC (XDR over TCP)
- **ポート**: ホストごとに個別ポート割当（16510, 16511, ...）
- **クライアント**: `digitalocean/go-libvirt` または `libvirt-go-module`
- **実装範囲**: Cirrusが使う約25のRPCプロシージャのみ
- **管理API**: REST (ポート8100) でホスト登録・設定変更

### ovn-sim

- **プロトコル**: OVSDB (RFC 7047, JSON-RPC over TCP)
- **ポート**: OVNクラスタごとに個別ポート（6641, 6642, ...）
- **スキーマ**: OVNの実際の `ovn-nb.ovsschema` を使用
- **実装範囲**: list_dbs, get_schema, transact, monitor, echo
- **管理API**: REST (ポート8200) でクラスタ登録・ポート状態制御

### storage-sim

- **プロトコル**: Cirrus Storage API (REST)
- **ポート**: 8500
- **マルチバックエンド**: `X-Backend-Id` ヘッダでバックエンド指定
- **管理API**: `/sim/` プレフィックスでバックエンド登録・設定
- **本番同等物**: storage-agent (Ceph agent, NFS agent等)

## マルチホスト/マルチバックエンドモデル

全シミュレータは1プロセスで多数のエンティティを管理する:

- **libvirt-sim**: ホストごとに異なるTCPポートでlibvirt RPCをリッスン。内部はホストID→インメモリ状態のマップ
- **ovn-sim**: OVNクラスタごとに異なるTCPポートでOVSDBをリッスン
- **storage-sim**: `X-Backend-Id` ヘッダでバックエンドを識別。単一ポートで全バックエンドを処理

この設計により、3000ホスト×50VM = 15万VMのシミュレーションが数百MBのメモリで動作する。

## 共通ライブラリ (common/)

全シミュレータが利用する横断的機能:

- **障害注入**: 確率的エラー、遅延、タイムアウト、部分障害をAPI経由で設定
- **イベントログ**: 全シミュレータのAPI呼び出しを時系列記録
- **状態スナップショット**: 全シミュレータの状態を保存・復元（テスト再現性）
- **データジェネレータ**: YAML定義から大規模環境を一発生成

## ライブマイグレーションのシミュレーション

libvirt-simの中で最も複雑な部分。

移行元ホスト（ポートA）と移行先ホスト（ポートB）間で、libvirtのマルチフェーズプロトコルを再現する:

1. Prepare (移行先): リソース予約
2. Perform (移行元): イテレーティブコピーのシミュレーション（タイマーで進捗管理）
3. Finish (移行先): ドメインをアクティブ化
4. Confirm (移行元): ドメインを除去

同一プロセス内なのでネットワーク通信は不要。内部的にホスト間の状態移動をインメモリで処理する。

## ディレクトリ構成方針

各シミュレータは以下の構造に従う:

```
{simulator}/
├── cmd/
│   └── main.go              エントリポイント
├── internal/
│   ├── server/              プロトコルサーバ実装
│   ├── state/               インメモリ状態管理
│   ├── handler/             リクエストハンドラ
│   └── sim/                 管理API（/sim/）
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

commonは共有ライブラリ:

```
common/
├── cmd/
│   └── main.go              共通APIサーバ
├── pkg/                     他シミュレータからimportされるパッケージ
│   ├── fault/               障害注入
│   ├── eventlog/            イベントログ
│   ├── snapshot/            状態スナップショット
│   └── datagen/             データジェネレータ
├── go.mod
└── Dockerfile
```
