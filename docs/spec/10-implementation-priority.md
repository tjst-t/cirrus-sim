# 10. 実装の優先順位

## Phase 1: 最小開発環境

目標: VM作成→ボリュームアタッチ→起動→停止→削除の基本フローが通る。

1. **libvirt-sim**: 管理API（ホスト登録）、libvirt RPCサブセット（接続管理、ドメインCRUD、状態遷移、リソース追跡、約15プロシージャ）
2. **storage-sim**: バックエンド登録、ボリュームCRUD、エクスポート/アンエクスポート、容量追跡
3. **common**: イベントログ
4. **docker-compose**: libvirt-sim, storage-sim, commonの起動

## Phase 2: ネットワークとマイグレーション

目標: VMにネットワーク接続、ライブマイグレーションが動作する。

5. **ovn-sim**: OVSDBプロトコル基盤（transact, monitor）、論理スイッチ、ポート、ルータ、ACL、DHCP、NAT、参照整合性
6. **libvirt-sim追加**: ライブマイグレーション（Perform/Prepare/Confirm/Finish）、イベント通知
7. **storage-sim追加**: スナップショット、クローン、依存関係管理、フラット化

## Phase 3: 運用機能

目標: ホストプロファイル適用、バックエンドドレインが動作する。

8. **awx-sim**: ジョブテンプレート、ジョブ実行、コールバック
9. **netbox-sim**: サイト/ラック/デバイスの階層
10. **storage-sim追加**: ストレージマイグレーション
11. **common追加**: 障害注入エンジン

## Phase 4: スケーラビリティ検証

目標: 数千ホスト規模での性能検証。

12. **common追加**: データジェネレータ、状態スナップショット/リストア
13. **ovn-sim追加**: OVN-ICシミュレーション
14. **load-gen**: ワークロード定義と実行
15. **environments**: small/medium/large環境定義
