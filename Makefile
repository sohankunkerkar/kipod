.PHONY: all build clean

REBUILD ?= false

all: build

build:
	@mkdir -p bin
	go build -o bin/kipod ./cmd/kipod

clean:
	rm -rf bin/

push-node-image: node-image
	podman tag localhost/kipod-node:latest $(REGISTRY)/kipod-node:$(IMAGE_TAG)
	podman push $(REGISTRY)/kipod-node:$(IMAGE_TAG)

# Patch local CRI-O source for development
# Usage: make patch-local-crio CRIO_SRC=/path/to/cri-o
patch-local-crio:
	@if [ -z "$(CRIO_SRC)" ]; then \
		echo "Error: CRIO_SRC is not set. Usage: make patch-local-crio CRIO_SRC=/path/to/cri-o"; \
		exit 1; \
	fi
	python3 images/base/patch_dbusmgr.py $(CRIO_SRC)/internal/dbusmgr/dbusmgr.go

node-image: build
	bin/kipod build node-image $(if $(filter true,$(REBUILD)),--rebuild)
