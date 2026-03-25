ovn-simのOVSDB実装に新しいテーブル $ARGUMENTS のサポートを追加してください。

1. `ovn-nb.ovsschema` から該当テーブルのスキーマ定義を読む
2. テーブルのカラム型、制約、参照整合性ルールを確認
3. インメモリストアにテーブルを追加
4. transactオペレーション（insert, select, update, delete, mutate）を実装
5. 参照整合性のバリデーションを実装
6. monitorで変更通知が正しく送信されることを確認
7. テストを追加（正常系＋参照整合性違反の異常系）
