.PHONY: all build test lint clean setup

SIMULATORS := libvirt-sim ovn-sim storage-sim awx-sim netbox-sim common load-gen webui

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

# Clean
clean:
	@for sim in $(SIMULATORS); do \
		if [ -f $$sim/go.mod ]; then \
			cd $$sim && go clean ./... && cd ..; \
		fi \
	done
