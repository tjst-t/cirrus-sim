# Cirrus-Sim ロードマップ

## 完了済み

### Phase 1: 最小開発環境 (完了)

VM作成→ボリュームアタッチ→起動→停止→削除の基本フローが通る。

- [x] libvirt-sim: 管理API、libvirt RPC（接続管理、ドメインCRUD、状態遷移、リソース追跡、26プロシージャ）
- [x] storage-sim: バックエンド登録、ボリュームCRUD、エクスポート/アンエクスポート、容量追跡
- [x] common: イベントログ

### Phase 2: ネットワークとマイグレーション (完了)

VMにネットワーク接続、ライブマイグレーションが動作する。

- [x] ovn-sim: OVSDBプロトコル（transact, monitor）、NB DB 13テーブル、参照整合性
- [x] libvirt-sim追加: ライブマイグレーション4フェーズ、イベント通知
- [x] storage-sim追加: スナップショット、クローン、依存関係管理、フラット化

### Phase 3: 運用機能 (完了)

外部システム連携のシミュレーション。

- [x] awx-sim: ジョブテンプレート、非同期ジョブ実行、コールバック
- [x] netbox-sim: サイト/ラック/デバイスの階層
- [x] storage-sim追加: ストレージマイグレーション（4フェーズ非同期）
- [x] common追加: 障害注入エンジン（error, delay, timeout, partial_failure）

### Phase 4: スケーラビリティ・統合 (完了)

環境生成と大規模テスト基盤。

- [x] common追加: データジェネレータ、状態スナップショット/リストア
- [x] ovn-sim追加: OVN-ICシミュレーション（IC NB DB 4テーブル）
- [x] load-gen: ワークロード定義と実行エンジン
- [x] environments: small(10)/medium(400)/large(2500+)環境定義

### Phase 5: 開発環境統合 (完了)

Cirrus開発のための統合環境。

- [x] 統一バイナリ `cirrus-sim`: 全シミュレータを1プロセスで起動
- [x] 環境シーディング: `-env` フラグでYAMLからホスト/クラスタ/バックエンド/物理トポロジを自動投入
- [x] embedded PostgreSQL: Cirrus本体のDBを組み込み起動
- [x] ダッシュボード Web UI: 全シミュレータの状態確認、詳細ドリルダウン、PostgreSQLテーブルブラウザ
- [x] portman連携: 全ポートの自動確保（管理API + ホストRPC + OVSDB + PostgreSQL）
- [x] `make serve` / `make stop` / `make deploy` による運用
- [x] netbox-sim NetBox v4互換: go-netbox v4クライアントで動作確認済み
- [x] libvirt-sim OVS連携: ドメインXMLのvirtualport/interfaceidパース
- [x] netbox-sim障害トポロジ: 階層ロケーション（サイト→フロア→ラック列）、障害共有属性
- [x] インテグレーションテスト: go-libvirt、go-netbox v4、OVSDB、AWX

## 今後の予定

### Phase 6: CLIツール・障害注入統合

Cirrus開発中の動作確認と障害テストを効率化する。

#### CLIツール (`cirrus-sim-ctl`)

- [ ] 状態確認コマンド
  - [ ] `status` — 全シミュレータの接続状態一覧
  - [ ] `hosts list` — libvirtホスト一覧（リソース使用状況付き）
  - [ ] `vms list [--host HOST]` — VM一覧
  - [ ] `backends list` — ストレージバックエンド一覧
  - [ ] `volumes list [--backend BACKEND]` — ボリューム一覧
  - [ ] `ovn clusters` — OVNクラスタ一覧
  - [ ] `ovn tables [--cluster CLUSTER]` — OVSDBテーブル/行数
  - [ ] `netbox sites` / `locations` / `racks` / `devices` — 物理トポロジ
  - [ ] `pg tables` — PostgreSQLテーブル一覧
  - [ ] `pg query "SQL"` — SQLクエリ実行
- [ ] 障害注入コマンド
  - [ ] `fault inject --target HOST --type error --operation OPERATION` — 障害ルール追加
  - [ ] `fault list` — 障害ルール一覧
  - [ ] `fault clear [ID]` — 障害ルール削除
- [ ] 状態管理コマンド
  - [ ] `snapshot save [NAME]` — 全シミュレータの状態を保存
  - [ ] `snapshot restore NAME` — 保存した状態を復元
  - [ ] `snapshot list` — スナップショット一覧
  - [ ] `reset` — 全状態リセット（PostgreSQL以外）
  - [ ] `export FILE` — 全状態をファイルにエクスポート
  - [ ] `import FILE` — ファイルから状態をインポート
- [ ] ポート自動検出（portman envファイルまたはデフォルトポート）

#### 障害注入の各シミュレータ統合

障害注入エンジン（common/pkg/fault）は実装済みだが、各シミュレータのハンドラがCheck()を呼んでいない。

- [ ] libvirt-sim: RPCハンドラでfault.Check()を呼び、マッチ時にエラー/遅延/タイムアウトを発生
- [ ] ovn-sim: transactハンドラでfault.Check()を呼ぶ
- [ ] storage-sim: APIハンドラでfault.Check()を呼ぶ
- [ ] awx-sim: ジョブ実行でfault.Check()を呼ぶ
- [ ] ダッシュボード: アクティブな障害ルールの表示

#### シミュレータ間整合性チェック

各シミュレータのAPIを外部から読み取って突合するチェッカー。設計詳細は `docs/CONSISTENCY_CHECK.md` を参照。

- [ ] 整合性チェックAPI（`GET /sim/consistency-check`）
- [ ] CLIコマンド（`cirrus-sim-ctl check`）
- [ ] ダッシュボードへの整合性ステータス表示

### Phase 7: 拡張機能（必要に応じて）

Cirrusの開発進捗に合わせて優先度を決定する。

- [ ] ストレージドメインの到達性シミュレーション（「このバックエンドはこのホスト群からアクセス可能」）
- [ ] イベントログの各シミュレータ自動連携（操作をcommonのイベントログに記録）
- [ ] libvirt-sim追加RPC（Cirrusが必要とするプロシージャを随時追加）
- [ ] OVN NB DBテーブル追加（Cirrusが必要とするテーブルを随時追加）
- [ ] netbox-sim CRUD API（現在は読み取り+bulk-loadのみ、個別のPOST/PUT/DELETEを追加）
