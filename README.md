# kipod – Kubernetes in rootless Podman with CRI‑O

**kipod** provides a simple CLI to spin up local Kubernetes clusters using **rootless Podman** containers as nodes and **CRI‑O** as the container runtime. It offers a `kind`‑like experience without Docker.

---

## Features

- 🚀 One‑command cluster creation (`kipod create cluster`).
- 📦 Pre‑built node image containing CRI‑O (rootless) and Kubernetes components.
- 🔒 Fully rootless – no privileged daemon required.
- 🛠️ Supports custom Kubernetes versions and image tags.

## Why Kipod?

While **kind** is excellent, it is designed for Docker and containerd. **kipod** is built specifically for the **Podman + CRI-O** ecosystem. It runs fully rootless by default, supports custom CRI-O builds out-of-the-box, and doesn't require a daemon, making it the ideal choice for Fedora/Red Hat users and CRI-O developers.

---

## Prerequisites

- Linux (tested on Fedora).
- **Podman** ≥ 4.0 (rootless mode enabled).
- **fuse‑overlayfs**.
- Subuid/Subgid ranges configured.
- Cgroup v2 with delegation.

---

## Installation

### From source
```bash
git clone https://github.com/sohankunkerkar/kipod.git
cd kipod
go build -o bin/kipod ./cmd/kipod
sudo mv bin/kipod /usr/local/bin/   # optional
```

---

## Quick start

1. Validate the environment:
```bash
kipod check
```
2. Build the node image (once or after changes):
```bash
kipod build node-image
```
3. Create a cluster:
```bash
kipod create cluster my‑cluster
```
4. Export kubeconfig (automatically printed; you can also use `--kubeconfig`):
```bash
export KUBECONFIG=~/.kube/my‑cluster-config
kubectl get nodes
```
5. Delete the cluster:
```bash
kipod delete cluster my‑cluster
```

## Configuration

Kipod supports declarative cluster configuration via YAML files. This allows you to customize cluster topology, runtime versions, and CRI-O settings.

### Basic Configuration

Create a configuration file (e.g., `my-cluster.yaml`):

```yaml
name: my-cluster
nodes:
  controlPlanes: 1
  workers: 2
cgroupManager: systemd
```

Create the cluster:

```bash
kipod create cluster --config my-cluster.yaml
```

### Configuration Options

#### Cluster Topology

```yaml
name: multi-node-cluster
nodes:
  controlPlanes: 1  # Number of control-plane nodes
  workers: 3        # Number of worker nodes
```

#### Component Versions

```yaml
versions:
  kubernetes: "1.34.2"  # Kubernetes version
  crio: "1.34"          # CRI-O minor version
  crun: "1.25"          # crun version
  runc: "1.3.3"         # runc version
```

#### Networking

```yaml
networking:
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"
  dnsDomain: "cluster.local"
```

#### Cgroup Manager

Choose between `cgroupfs` (default, rootless-friendly) or `systemd`:

```yaml
cgroupManager: systemd  # or "cgroupfs"
```

#### Storage

Configure container storage backend. Use `volume` for persistence and advanced features (like Spegel), or `tmpfs` (default) for speed and simplicity.

```yaml
storage:
  type: volume  # or "tmpfs"
  # size: 20G   # optional, mostly for tmpfs
```

### Advanced: Custom CRI-O Binary

For CRI-O development, you can use a locally-built CRI-O binary:

```yaml
name: crio-dev
localBuilds:
  crioBinary: /path/to/cri-o/bin/crio
```

**Example workflow:**

```bash
# 1. Patch and build CRI-O
cd ~/dev/cri-o
make patch-local-crio CRIO_SRC=$(pwd)
make bin/crio

# 2. Create cluster with custom binary
cat > dev-cluster.yaml <<EOF
name: crio-dev
localBuilds:
  crioBinary: $HOME/dev/cri-o/bin/crio
EOF

kipod create cluster --config dev-cluster.yaml
```

### Advanced: Custom CRI-O Configuration

Inject custom CRI-O configuration for features like blob caching (Spegel integration):

**1. Create CRI-O config file** (`crio-custom.conf`):

```toml
[crio.image]
enable_blob_cache = true

[crio.runtime]
log_level = "debug"
```

**2. Reference in cluster config:**

```yaml
name: spegel-cluster
nodes:
  controlPlanes: 1
  workers: 2
cgroupManager: systemd
crioConfig: ./crio-custom.conf
```

**3. Create cluster:**

```bash
kipod create cluster --config spegel-cluster.yaml
```

The config file will be mounted and applied to `/etc/crio/crio.conf.d/99-user.conf` on all nodes.

### Complete Example

```yaml
name: production-test
nodes:
  controlPlanes: 1
  workers: 3
versions:
  kubernetes: "1.34.2"
  crio: "1.34"
networking:
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"
cgroupManager: systemd
crioConfig: ./configs/crio-production.conf
localBuilds:
  crioBinary: /home/user/dev/cri-o/bin/crio
```

See `examples/` directory for more configuration samples.

## Examples

---

## Image publishing

```makefile
# Adjust REGISTRY to your Quay namespace
REGISTRY ?= quay.io/<namespace>/kipod
IMAGE_TAG ?= latest

push-node-image: node-image
	podman tag localhost/kipod-node:latest $(REGISTRY)/kipod-node:$(IMAGE_TAG)
	podman push $(REGISTRY)/kipod-node:$(IMAGE_TAG)
```

Publish the latest image:
```bash
make push-node-image
```
Or publish a specific Kubernetes version:
```bash
make push-node-image-version K8S_VERSION=v1.34.2
```

---

## Commands reference

| Command | Description |
|---------|-------------|
| `kipod check` | Verify system prerequisites |
| `kipod build node-image [--k8s-version X]` | Build the node image |
| `kipod create cluster [NAME] [--wait DURATION] [--retain] [--kubeconfig PATH]` | Create a cluster |
| `kipod delete cluster [NAME]` | Delete a cluster |
| `kipod get clusters` | List existing clusters |

---

## License

Apache License 2.0.

---

## Acknowledgments

- Inspired by **kind** – https://kind.sigs.k8s.io/
- Built for the Podman and CRI‑O communities.
- Thanks to the developers of **Podman**, **CRI‑O**, and **Kubernetes**.
