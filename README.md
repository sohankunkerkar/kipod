# kipod - Kubernetes in Podman with CRI-O

**kipod** is a tool for running local Kubernetes clusters using rootless Podman containers as nodes with CRI-O as the container runtime.

Inspired by [kind](https://kind.sigs.k8s.io/), but designed specifically for **rootless Podman** and **CRI-O** instead of Docker and containerd.

## Features

- 🚀 Quick local Kubernetes cluster creation using rootless Podman
- 📦 Uses CRI-O as the container runtime (instead of containerd)
- 🔒 Runs in rootless mode for better security
- 🐧 Works on Linux systems with Podman installed
- 🎯 Simple CLI interface
- 🔧 Perfect for local development and testing

## Prerequisites

- **Linux system** (tested on Fedora/Ubuntu)
- **Podman** installed (version 4.0+)
- **fuse-overlayfs** installed
- **User namespaces** enabled
- **Cgroup v2** with delegation
- **Proper subuid/subgid configuration**

## Installation

### From Source

```bash
git clone https://github.com/skunkerk/kipod.git
cd kipod
go build -o kipod ./cmd/kipod
sudo mv kipod /usr/local/bin/
```

## Quick Start

### 1. Check Prerequisites

First, validate your system meets the requirements:

```bash
kipod check
```

This will check for:
- Podman installation
- User namespaces configuration
- subuid/subgid setup
- Cgroup v2
- FUSE support
- Cgroup delegation

### 2. Configure Your System (if needed)

If the check fails, you may need to configure your system:

#### Configure subuid/subgid

Add your user to `/etc/subuid` and `/etc/subgid`:

```bash
sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 $USER
```

Or manually edit the files:

```bash
echo "$USER:100000:65536" | sudo tee -a /etc/subuid
echo "$USER:100000:65536" | sudo tee -a /etc/subgid
```

#### Enable user namespaces

```bash
sudo sysctl -w kernel.unprivileged_userns_clone=1
sudo sysctl -w user.max_user_namespaces=28633
```

To make these persistent:

```bash
echo "kernel.unprivileged_userns_clone=1" | sudo tee -a /etc/sysctl.d/99-userns.conf
echo "user.max_user_namespaces=28633" | sudo tee -a /etc/sysctl.d/99-userns.conf
```

#### Enable cgroup delegation (systemd)

Create `/etc/systemd/system/user@.service.d/delegate.conf`:

```bash
sudo mkdir -p /etc/systemd/system/user@.service.d/
cat << EOF | sudo tee /etc/systemd/system/user@.service.d/delegate.conf
[Service]
Delegate=yes
EOF

sudo systemctl daemon-reload
```

Log out and log back in for changes to take effect.

### 3. Build Node Image

Before creating clusters, build the node image:

```bash
kipod build node-image
```

This builds a container image with:
- Fedora base
- Systemd
- CRI-O configured for rootless/nested containers
- Kubernetes components (kubelet, kubeadm, kubectl)
- CNI plugins

Building takes 5-10 minutes depending on your connection.

### 4. Create a Cluster

```bash
kipod create cluster
```

This creates a single-node Kubernetes cluster named "kipod" by default.

### Create a Named Cluster

```bash
kipod create cluster my-cluster
```

### 5. Access Your Cluster

```bash
# Get kubeconfig from the cluster
podman exec kipod-control-plane-0 cat /etc/kubernetes/admin.conf > ~/.kube/kipod-config

# Use it
export KUBECONFIG=~/.kube/kipod-config
kubectl get nodes
kubectl get pods -A
```

### 6. Delete a Cluster

```bash
kipod delete cluster my-cluster
```

## Architecture

kipod creates Kubernetes clusters by:

1. **Building a node image** - Pre-installs CRI-O and Kubernetes in a container image
2. **Creating rootless Podman containers** - Each container acts as a Kubernetes node
3. **Configuring for nested containers** - Uses fuse-overlayfs, user namespaces, cgroup delegation
4. **Initializing the cluster** - Uses kubeadm to bootstrap the cluster

### How it differs from kind

| Feature | kind | kipod |
|---------|------|-------|
| Container Engine | Docker | **Podman (rootless)** |
| Container Runtime | containerd | **CRI-O** |
| Privileges | Requires Docker daemon | **Rootless** |
| Storage Driver | overlay | **VFS** |
| Cgroup Manager | systemd | **cgroupfs** |
| OOM Score Handling | Native | **crun wrapper** |
| Use Case | Docker environments | Rootless Podman/CRI-O environments |

### Nested Containers Challenge

Running CRI-O inside a rootless Podman container is complex because:
- **User namespaces** must be properly mapped
- **Cgroup delegation** is required for resource management
- **Storage driver** - VFS needed to avoid overlay whiteout issues in nested containers
- **OOM score adjustment** - rootless containers cannot lower oomScoreAdj without privileges
- **Cgroup manager** - systemd requires D-Bus which isn't available in nested containers
- **Sysctl settings** - many kernel parameters cannot be set in rootless mode
- **/dev/fuse** access is needed
- **Systemd** must work inside the container

kipod handles all these complexities automatically with:
- Custom crun wrapper for OOM score clamping
- cgroupfs cgroup manager (no D-Bus dependency)
- VFS storage driver (no overlay issues)
- kube-proxy configuration to skip privileged sysctls

## Commands Reference

### check

```bash
kipod check
```

Validates system prerequisites for running kipod.

### build node-image

```bash
kipod build node-image [flags]
```

Builds a kipod node image.

Flags:
- `--k8s-version` - Kubernetes version (default: "1.28")
- `--crio-version` - CRI-O version (default: "1.28")
- `--image-name` - Image name (default: "localhost/kipod-node")
- `--image-tag` - Image tag (default: "latest")

### create cluster

```bash
kipod create cluster [name]
```

Creates a Kubernetes cluster.

### delete cluster

```bash
kipod delete cluster [name]
```

Deletes a Kubernetes cluster.

### get clusters

```bash
kipod get clusters
```

Lists all kipod clusters.

## Usage Examples

### Check CRI-O status

```bash
podman exec kipod-control-plane-0 systemctl status crio
podman exec kipod-control-plane-0 crictl info
podman exec kipod-control-plane-0 crictl ps
```

### Deploy a test application

```bash
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --port=80 --type=NodePort
kubectl get pods
kubectl get svc
```

### Exec into a node

```bash
podman exec -it kipod-control-plane-0 bash
```

### View logs

```bash
# CRI-O logs
podman exec kipod-control-plane-0 journalctl -u crio -f

# Kubelet logs
podman exec kipod-control-plane-0 journalctl -u kubelet -f
```

## Troubleshooting

### Check command fails

Run `kipod check` and fix any issues reported. Most common issues:
- Missing subuid/subgid configuration
- User namespaces not enabled
- Cgroup delegation not configured

### Node image build fails

Check that:
- Podman is installed and working
- You have internet connectivity
- You have sufficient disk space

### Cluster creation fails

Check:
- Node image exists: `podman images | grep kipod-node`
- Podman can create rootless containers
- System prerequisites: `kipod check`

### CRI-O not starting

```bash
podman exec kipod-control-plane-0 journalctl -u crio -n 100
```

Common issues:
- fuse-overlayfs not working
- User namespace mapping issues
- Cgroup permission problems

### Kubernetes not initializing

```bash
podman exec kipod-control-plane-0 journalctl -u kubelet -n 100
podman exec kipod-control-plane-0 kubeadm reset -f  # Reset and try again
```

## Project Structure

```
kipod/
├── cmd/
│   └── kipod/           # CLI entry point
├── pkg/
│   ├── cluster/         # Cluster creation and management
│   ├── podman/          # Podman container operations (rootless-aware)
│   ├── crio/            # CRI-O configuration
│   ├── build/           # Node image builder
│   ├── system/          # System validation
│   └── config/          # Configuration types
├── images/
│   └── base/            # Node image Containerfile
└── README.md
```

## Development Status

This is a **working prototype** for rootless Kubernetes with Podman and CRI-O.

Current features:
- ✅ Rootless Podman support
- ✅ Pre-built node images with crun OOM score wrapper
- ✅ Single-node cluster creation
- ✅ CRI-O with VFS storage driver (cgroupfs cgroup manager)
- ✅ System validation
- ✅ Cluster deletion
- ✅ Fully functional networking (kube-proxy + CoreDNS)
- ✅ Workload scheduling and execution

Planned features:
- ⏳ Multi-node clusters
- ⏳ Configuration file support
- ⏳ Multiple Kubernetes versions
- ⏳ Custom CNI plugins
- ⏳ Load balancer for multi-master
- ⏳ Image pre-loading

## Contributing

Contributions are welcome! This is an early prototype for bringing Kubernetes to rootless Podman environments.

## License

Apache 2.0 (following kind's licensing)

## Acknowledgments

- Inspired by [kind](https://kind.sigs.k8s.io/)
- Built for the rootless Podman and CRI-O communities
- Thanks to the Kubernetes, Podman, and CRI-O projects
- Special thanks to the fuse-overlayfs project for making rootless nested containers possible

## Why kipod?

- **k** - Kubernetes
- **i** - in
- **pod** - Podman

Plus, "pod" is a Kubernetes concept, making it a fitting name!

## Technical Deep Dive

### Rootless Nested Containers

Running containers inside rootless containers requires:

1. **User namespace mapping** - Proper subuid/subgid ranges
2. **Cgroup delegation** - Systemd must delegate cgroups to user session
3. **Storage driver** - fuse-overlayfs instead of overlay
4. **Device access** - /dev/fuse must be accessible
5. **Security** - Unmasking /sys/fs/cgroup and /proc/*

kipod configures all of these automatically when creating node containers.

### CRI-O Configuration

The node image configures CRI-O to use:
- **Storage driver**: VFS (avoids overlay whiteout file issues in nested containers)
- **Cgroup manager**: cgroupfs (works without D-Bus in nested containers)
- **Runtime**: crun with custom wrapper for OOM score adjustment
- **Network**: CNI with bridge plugin

### Rootless Container Fixes

Running Kubernetes in rootless nested containers required several key fixes:

#### 1. OOM Score Adjustment (crun wrapper)
- **Problem**: Containers cannot set `oom_score_adj` lower than parent process in rootless mode
- **Solution**: Created a crun wrapper (`images/base/crun-wrapper.sh`) that:
  - Intercepts crun calls and reads the OCI config.json
  - Clamps `oomScoreAdj` values to valid ranges for rootless containers
  - Allows kubelet to function without permission errors

#### 2. Cgroup Manager (cgroupfs)
- **Problem**: systemd cgroup manager requires D-Bus socket, unavailable in nested containers
- **Solution**: Configured both CRI-O and kubelet to use cgroupfs cgroup manager
  - No D-Bus dependency
  - Works reliably in nested container environments

#### 3. kube-proxy Conntrack Configuration
- **Problem**: kube-proxy tries to set sysctl values that require privileges
- **Solution**: Configure kube-proxy with `conntrack.maxPerCore: 0` to skip privileged sysctl operations

These fixes enable fully functional Kubernetes clusters in rootless Podman containers!

---

**Note**: This is a prototype tool for local development in rootless environments. For production Kubernetes clusters, use proper cluster management tools.
