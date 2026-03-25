.PHONY: all build test lint clean setup serve stop logs

SIMULATORS := libvirt-sim ovn-sim storage-sim awx-sim netbox-sim common load-gen webui

# Paths for serve target (background mode with portman)
PID_FILE := /tmp/cirrus-sim-dev.pid
LOG_FILE := /tmp/cirrus-sim-dev.log
PORTMAN_ENV := /tmp/cirrus-sim-portman.env

all: lint test build

# Setup development tools
setup:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Build all simulators
build:
	@for sim in $(SIMULATORS); do \
		if [ -f $$sim/go.mod ]; then \
			echo "Building $$sim..."; \
			cd $$sim && go build ./... && cd ..; \
		fi \
	done

# Build individual simulators
define build_target
build-$(1):
	cd $(1) && go build ./...
endef
$(foreach sim,$(SIMULATORS),$(eval $(call build_target,$(sim))))

# Test all simulators
test:
	@for sim in $(SIMULATORS); do \
		if [ -f $$sim/go.mod ]; then \
			echo "Testing $$sim..."; \
			cd $$sim && go test ./... && cd ..; \
		fi \
	done

# Test individual simulators
define test_target
test-$(1):
	cd $(1) && go test ./... -v
endef
$(foreach sim,$(SIMULATORS),$(eval $(call test_target,$(sim))))

# Lint all simulators
lint:
	@for sim in $(SIMULATORS); do \
		if [ -f $$sim/go.mod ]; then \
			echo "Linting $$sim..."; \
			cd $$sim && golangci-lint run ./... && cd ..; \
		fi \
	done

# Integration tests (requires docker-compose up)
test-integration:
	cd tests/integration && go test ./... -v -tags=integration

# Docker
up:
	docker-compose up -d

down:
	docker-compose down

up-testing:
	docker-compose --profile testing up -d

# ── Serve (portman-managed, background) ──

serve: build-unified
	@if [ -f $(PID_FILE) ]; then \
	  OLD_PID=$$(cat $(PID_FILE)); \
	  if kill -0 $$OLD_PID 2>/dev/null; then \
	    echo "==> Killing previous process (PID: $$OLD_PID)..."; \
	    kill $$OLD_PID; \
	    for i in $$(seq 1 50); do kill -0 $$OLD_PID 2>/dev/null || break; sleep 0.1; done; \
	    kill -0 $$OLD_PID 2>/dev/null && kill -9 $$OLD_PID 2>/dev/null || true; \
	  fi; \
	  rm -f $(PID_FILE); \
	fi
	@portman env \
	  --name common \
	  --name dashboard --expose \
	  --name libvirt-sim \
	  --name ovn-sim \
	  --name awx-sim \
	  --name netbox-sim \
	  --name storage-sim \
	  --output $(PORTMAN_ENV)
	@bash -c '\
	  set -a; source $(PORTMAN_ENV); set +a; \
	  echo "==> Starting cirrus-sim (log: $(LOG_FILE))"; \
	  nohup ./bin/cirrus-sim \
	    -common=$$COMMON_PORT \
	    -dashboard=$$DASHBOARD_PORT \
	    -libvirt=$$LIBVIRT_SIM_PORT \
	    -ovn=$$OVN_SIM_PORT \
	    -awx=$$AWX_SIM_PORT \
	    -netbox=$$NETBOX_SIM_PORT \
	    -storage=$$STORAGE_SIM_PORT \
	    > $(LOG_FILE) 2>&1 & \
	  echo $$! > $(PID_FILE); \
	  sleep 1; \
	  echo ""; \
	  echo "  cirrus-sim is running (PID: $$(cat $(PID_FILE)))"; \
	  echo "  ─────────────────────────────────────────"; \
	  echo "  Dashboard                http://localhost:$$DASHBOARD_PORT"; \
	  echo "  ─────────────────────────────────────────"; \
	  echo "  common (events/faults)   http://localhost:$$COMMON_PORT"; \
	  echo "  libvirt-sim (management) http://localhost:$$LIBVIRT_SIM_PORT"; \
	  echo "  ovn-sim (management)     http://localhost:$$OVN_SIM_PORT"; \
	  echo "  awx-sim                  http://localhost:$$AWX_SIM_PORT"; \
	  echo "  netbox-sim               http://localhost:$$NETBOX_SIM_PORT"; \
	  echo "  storage-sim              http://localhost:$$STORAGE_SIM_PORT"; \
	  echo "  ─────────────────────────────────────────"; \
	  echo "  Log: $(LOG_FILE)"; \
	  echo "  Stop: make stop"'

build-unified:
	@echo "Building cirrus-sim..."
	@mkdir -p bin
	@cd cmd/cirrus-sim && go build -o ../../bin/cirrus-sim .

stop:
	@if [ -f $(PID_FILE) ]; then \
	  PID=$$(cat $(PID_FILE)); \
	  if kill -0 $$PID 2>/dev/null; then \
	    echo "==> Stopping cirrus-sim (PID: $$PID)..."; \
	    kill $$PID; \
	    for i in $$(seq 1 50); do kill -0 $$PID 2>/dev/null || break; sleep 0.1; done; \
	    kill -0 $$PID 2>/dev/null && kill -9 $$PID 2>/dev/null || true; \
	    echo "    Stopped."; \
	  else \
	    echo "Process $$PID not running."; \
	  fi; \
	  rm -f $(PID_FILE); \
	else \
	  echo "No PID file found. cirrus-sim is not running."; \
	fi

logs:
	@if [ -f $(LOG_FILE) ]; then \
	  tail -f $(LOG_FILE); \
	else \
	  echo "No log file found at $(LOG_FILE)"; \
	fi

# Clean
clean:
	@for sim in $(SIMULATORS); do \
		if [ -f $$sim/go.mod ]; then \
			cd $$sim && go clean ./... && cd ..; \
		fi \
	done
