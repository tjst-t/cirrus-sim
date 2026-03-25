# 開発ガイド

## 前提条件

- Go 1.22+
- Docker & Docker Compose
- golangci-lint
- make

## セットアップ

```bash
git clone https://github.com/tjst-t/cirrus-sim.git
cd cirrus-sim
make setup   # ツールのインストール
make build   # 全ビルド
make test    # 全テスト
```

## 新しいシミュレータの追加

1. ディレクトリを作成: `mkdir -p {name}-sim/cmd {name}-sim/internal`
2. `go mod init` で独立モジュールとして初期化
3. `docs/ARCHITECTURE.md` のディレクトリ構成方針に従う
4. `docker-compose.yml` にサービスを追加
5. `Makefile` にビルド・テストターゲットを追加

## テスト方針

### ユニットテスト

各パッケージに `_test.go` を配置。テーブル駆動テストを基本とする。

```go
func TestDomainStateTransition(t *testing.T) {
    tests := []struct {
        name      string
        initial   DomainState
        action    Action
        expected  DomainState
        wantErr   bool
    }{
        {"start from shutoff", Shutoff, Start, Running, false},
        {"start from running", Running, Start, Running, true},
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### インテグレーションテスト

`tests/integration/` にCirrusとシミュレータ間の結合テストを配置。
docker-composeで全シミュレータを起動した状態で実行。

```bash
make test-integration
```

## Git規約

### ブランチ

- `main`: 安定版
- `feature/{simulator}-{feature}`: 機能開発（例: `feature/libvirt-sim-migration`）
- `fix/{simulator}-{description}`: バグ修正

### コミットメッセージ

[Conventional Commits](https://www.conventionalcommits.org/) に従う:

```
feat(libvirt-sim): add domain state transition logic
fix(storage-sim): fix capacity tracking for thin provisioned volumes
test(ovn-sim): add OVSDB transact operation tests
docs: update SPEC.md with storage migration API
chore: update go dependencies
```

スコープはシミュレータ名（`libvirt-sim`, `ovn-sim`, `storage-sim`, `awx-sim`, `netbox-sim`, `common`, `load-gen`）。

## CI/CD

GitHub Actionsで以下を実行:

- `make lint` — プッシュごと
- `make test` — プッシュごと
- `make build` — プッシュごと
- `make test-integration` — mainへのPRマージ前
