# 1. 概要

## 1.1 目的

Cirrus IaaSの開発・テストに必要な外部依存システムのシミュレータ群。Cirrusが本物の外部システムと通信するのと同じプロトコルで通信し、実際のハードウェアやインフラなしにCirrusの全機能を開発・テストできるようにする。

## 1.2 プロトコル方針

全シミュレータは本番環境と同一のプロトコルを喋る。Cirrus本体のコードは接続先エンドポイントの設定変更のみでシミュレータと本番を切り替えられる。

```
libvirt:   Cirrus → libvirt RPC (XDR)         → libvirtd (本番) / libvirt-sim (開発)
OVN:       Cirrus → OVSDB (JSON-RPC)          → ovsdb-server (本番) / ovn-sim (開発)
Storage:   Cirrus → Cirrus Storage API (REST)  → storage-agent (本番) / storage-sim (開発)
AWX:       Cirrus → AWX REST API               → AWX (本番) / awx-sim (開発)
NetBox:    Cirrus → NetBox REST API            → NetBox (本番) / netbox-sim (開発)
```

ストレージのみCirrus独自のREST APIを定義する。libvirtやOVNには業界標準プロトコルがあるが、ストレージにはボリューム管理の統一プロトコルが存在しないため。本番ではバックエンドごとのstorage-agent（Ceph agent、NFS agent等）がこのAPIを実装し、内部でlibrbd/NFS操作等に変換する。

## 1.3 リポジトリ構成

```
cirrus-sim/
├── common/              ← 共有ライブラリ
│   ├── fault_injection/   障害注入エンジン
│   ├── latency/           レイテンシ注入
│   ├── event_log/         イベントログ
│   └── data_generator/    大規模環境データ生成
├── libvirt-sim/         ← libvirtシミュレータ（libvirt RPCプロトコル）
├── ovn-sim/             ← OVN Northbound DBシミュレータ（OVSDBプロトコル）
├── storage-sim/         ← ストレージシミュレータ（Cirrus Storage API）
├── awx-sim/             ← AWX hookシミュレータ
├── netbox-sim/          ← CMDB同期シミュレータ
├── load-gen/            ← 負荷ジェネレータ
├── docker-compose.yml   ← 全シミュレータ一発起動
└── environments/        ← 環境定義ファイル
    ├── small.yaml         10ホスト開発環境
    ├── medium.yaml        500ホスト検証環境
    └── large.yaml         3000ホスト負荷テスト環境
```

## 1.4 設計原則

- 全シミュレータが本番と同一プロトコルで通信
- 1プロセスで数千ホスト分をシミュレーション可能（マルチホストモード）
- 全シミュレータがインメモリで動作し、外部DBやストレージ不要
- 障害注入・レイテンシ注入・イベントログを全シミュレータで共通利用
- 状態のスナップショット・リストアによるテスト再現性の確保
- docker-compose一発起動による開発体験の最適化
