# Cirrus-Sim

Simulator suite for [Cirrus IaaS](https://github.com/tjst-t/cirrus) development. Each simulator speaks the same protocol as its production counterpart, enabling full-stack IaaS development and testing without physical infrastructure.

## Simulators

| Simulator | Protocol | Port | Description |
|-----------|----------|------|-------------|
| libvirt-sim | libvirt RPC (XDR/TCP) | 16509+ | Compute host simulation (VM lifecycle, migration, resources) |
| ovn-sim | OVSDB (JSON-RPC/TCP) | 6641+ | OVN Northbound DB (virtual networking) |
| storage-sim | Cirrus Storage API (REST) | 8500 | Block storage backend (volumes, snapshots, clones) |
| awx-sim | AWX REST API | 8300 | Ansible AWX job execution |
| netbox-sim | NetBox REST API | 8400 | DCIM/CMDB (physical topology) |
| common | REST | 8000 | Shared services (fault injection, event log, data generator) |
| load-gen | REST | 8600 | Load testing and benchmarks |

## Quick Start

```bash
# Start all simulators
docker-compose up -d

# Load a small test environment (10 hosts)
curl -X POST http://localhost:8000/api/v1/generate \
  -H "Content-Type: application/yaml" \
  --data-binary @environments/small.yaml

# Start with load generator
docker-compose --profile testing up -d
```

## Development

```bash
# Build all
make build

# Test all
make test

# Lint
make lint

# Build individual simulator
make build-libvirt-sim
make build-ovn-sim
make build-storage-sim
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the overall design.

## Specification

See [docs/SPEC.md](docs/SPEC.md) for the detailed simulator specification.

## Implementation Phases

1. **Phase 1** — Minimal dev environment: libvirt-sim (domain CRUD), storage-sim (volume CRUD), common (event log)
2. **Phase 2** — Networking & migration: ovn-sim (OVSDB), live migration, snapshots/clones
3. **Phase 3** — Operations: awx-sim, netbox-sim, storage migration, fault injection
4. **Phase 4** — Scale testing: data generator, load-gen, large environments

## License

TBD
