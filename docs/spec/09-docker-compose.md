# 9. docker-compose構成

```yaml
version: "3.8"

services:
  libvirt-sim:
    build: ./libvirt-sim
    ports:
      - "8100:8100"                    # 管理API
      - "16509-16609:16509-16609"      # libvirt RPC
    environment:
      - LOG_LEVEL=info
      - EVENT_LOG_ENDPOINT=http://common-api:8000

  ovn-sim:
    build: ./ovn-sim
    ports:
      - "8200:8200"                    # 管理API
      - "6641-6649:6641-6649"          # OVSDB
    environment:
      - LOG_LEVEL=info
      - EVENT_LOG_ENDPOINT=http://common-api:8000

  storage-sim:
    build: ./storage-sim
    ports:
      - "8500:8500"                    # Cirrus Storage API + 管理API
    environment:
      - LOG_LEVEL=info
      - EVENT_LOG_ENDPOINT=http://common-api:8000

  awx-sim:
    build: ./awx-sim
    ports:
      - "8300:8300"
    environment:
      - LOG_LEVEL=info
      - EVENT_LOG_ENDPOINT=http://common-api:8000

  netbox-sim:
    build: ./netbox-sim
    ports:
      - "8400:8400"
    environment:
      - LOG_LEVEL=info
      - EVENT_LOG_ENDPOINT=http://common-api:8000

  common-api:
    build: ./common
    ports:
      - "8000:8000"
    environment:
      - LOG_LEVEL=info
    volumes:
      - ./environments:/environments

  load-gen:
    build: ./load-gen
    ports:
      - "8600:8600"
    environment:
      - CIRRUS_ENDPOINT=http://cirrus:8080
      - FAULT_INJECTION_ENDPOINT=http://common-api:8000
    profiles:
      - testing
```

## 起動コマンド

```bash
# 開発用（シミュレータのみ）
docker-compose up -d

# テスト用（負荷ジェネレータ含む）
docker-compose --profile testing up -d

# 環境データ投入
curl -X POST http://localhost:8000/api/v1/generate \
  -H "Content-Type: application/yaml" \
  --data-binary @environments/small.yaml
```
