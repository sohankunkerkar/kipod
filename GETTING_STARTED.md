# Getting Started with kipod

## What We Built

**kipod** (Kubernetes In Podman) is a tool for running local Kubernetes clusters using **rootless Podman** containers with **CRI-O** as the container runtime. This is similar to kind (Kubernetes in Docker), but specifically designed for rootless environments.

## Key Differences from kind

- **Rootless by design**: Runs without root privileges using rootless Podman
- **CRI-O runtime**: Uses CRI-O instead of containerd
- **Pre-built images**: Node images are built ahead of time with everything pre-installed
- **Nested containers**: Handles the complexity of running CRI-O inside Podman containers

## Project Structure

```
/home/skunkerk/dev/kipod/
├── cmd/kipod/               # CLI implementation
│   ├── main.go             # Main CLI and command structure
│   ├── cluster.go          # Cluster creation/deletion commands
│   ├── build.go            # Build command implementation
│   └── check.go            # System check command
├── pkg/
│   ├── build/              # Node image builder
│   │   └── image.go        # Image building logic
│   ├── cluster/            # Cluster management
│   │   └── cluster.go      # Cluster creation/deletion logic
│   ├── podman/             # Podman wrapper
│   │   └── podman.go       # Rootless-aware podman operations
│   ├── crio/               # CRI-O configuration
│   │   └── config.go       # CRI-O config generation
│   ├── system/             # System validation
│   │   └── validate.go     # Prerequisites checking
│   └── config/             # Configuration types
│       └── types.go        # Config structures
├── images/base/            # Node image definition
│   ├── Containerfile       # Multi-stage build for node image
│   └── entrypoint.sh       # Container entrypoint script
├── README.md               # Comprehensive documentation
├── GETTING_STARTED.md      # This file
└── go.mod                  # Go dependencies
```

## How It Works

### 1. System Prerequisites

kipod requires specific system configuration for rootless nested containers:

- **subuid/subgid**: User namespace UID/GID mapping
- **User namespaces**: Kernel support for unprivileged user namespaces
- **Cgroup v2**: Modern cgroup interface with delegation
- **fuse-overlayfs**: Storage driver for rootless overlay mounts
- **/dev/fuse**: Device access for fuse operations

Run `kipod check` to validate your system.

### 2. Node Image Building

The `kipod build node-image` command creates a container image with:

**Base**: Fedora 39 (for good systemd/CRI-O support)

**Installed Components**:
- Systemd (configured for containers)
- CRI-O with fuse-overlayfs storage driver
- Kubernetes components (kubelet, kubeadm, kubectl)
- CNI plugins
- Required tools (crictl, conntrack, etc.)

**Special Configuration**:
- CRI-O configured to use fuse-overlayfs (for rootless)
- Systemd configured to run in a container
- Cgroup delegation setup
- User namespace aware configuration

### 3. Cluster Creation

When you run `kipod create cluster`, it:

1. **Validates** the node image exists
2. **Creates** a rootless Podman container with special flags:
   - `--systemd=always` - Enable systemd mode
   - `--cgroupns=private` - Private cgroup namespace
   - `--security-opt unmask=/sys/fs/cgroup` - Allow cgroup access
   - `--device /dev/fuse` - Enable fuse-overlayfs
   - Mounts `/sys/fs/cgroup` as read-write
3. **Waits** for systemd and CRI-O to start
4. **Initializes** Kubernetes using kubeadm
5. **Configures** the cluster for single-node operation

### 4. Rootless Container Flags

The key to making this work is the special Podman flags in `pkg/podman/podman.go`:

```go
if opts.Rootless {
    args = append(args, "--systemd=always")
    args = append(args, "--cgroupns=private")
    args = append(args, "--security-opt", "unmask=/sys/fs/cgroup")
    args = append(args, "--security-opt", "unmask=/proc/*")
    args = append(args, "--device", "/dev/fuse")
    args = append(args, "-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw")
    args = append(args, "--tmpfs", "/run")
    args = append(args, "--tmpfs", "/tmp")
}
```

These flags enable:
- Systemd to work inside the container
- CRI-O to manage cgroups
- fuse-overlayfs for nested container storage
- Proper /proc and /sys access

## Next Steps

### To Test kipod:

1. **Check your system**:
   ```bash
   /home/skunkerk/dev/kipod/kipod check
   ```

2. **Configure your system** (if checks fail):
   - Set up subuid/subgid: `sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 $USER`
   - Enable user namespaces: `sudo sysctl -w user.max_user_namespaces=28633`
   - Configure cgroup delegation (see README.md)
   - Log out and log back in

3. **Build the node image**:
   ```bash
   /home/skunkerk/dev/kipod/kipod build node-image
   ```
   This will take 5-10 minutes.

4. **Create a test cluster**:
   ```bash
   /home/skunkerk/dev/kipod/kipod create cluster test
   ```

5. **Access the cluster**:
   ```bash
   podman exec test-control-plane-0 cat /etc/kubernetes/admin.conf > /tmp/kipod-kubeconfig
   export KUBECONFIG=/tmp/kipod-kubeconfig
   kubectl get nodes
   kubectl get pods -A
   ```

6. **Clean up**:
   ```bash
   /home/skunkerk/dev/kipod/kipod delete cluster test
   ```

## Debugging Tips

### Container won't start
```bash
podman logs <container-id>
```

### CRI-O issues
```bash
podman exec <container-name> journalctl -u crio -n 100
podman exec <container-name> crictl info
```

### Kubelet issues
```bash
podman exec <container-name> journalctl -u kubelet -n 100
```

### Check cgroup setup
```bash
podman exec <container-name> cat /sys/fs/cgroup/cgroup.controllers
podman exec <container-name> systemctl status
```

### View full kubeadm output
The cluster creation shows abbreviated output. For full logs:
```bash
podman exec <container-name> journalctl -xe
```

## Architecture Decisions

### Why Pre-built Images?

Unlike the initial design which tried to install CRI-O at runtime, pre-built images:
- Are faster (installation happens once)
- Are more reliable (tested configurations)
- Are reproducible (same image = same behavior)
- Follow kind's proven pattern

### Why Rootless?

Rootless Podman provides:
- Better security (no root daemon)
- User namespace isolation
- No special permissions needed
- Suitable for development environments

### Why CRI-O?

CRI-O is:
- Kubernetes-native (designed for Kubernetes)
- Lightweight (only what Kubernetes needs)
- OCI compliant
- Well-supported in Fedora/RHEL ecosystems

### Challenges Overcome

1. **Nested containers**: Solved with fuse-overlayfs and proper device access
2. **Cgroup management**: Solved with cgroup v2 delegation and unmasking
3. **Systemd in container**: Solved with `--systemd=always` and proper tmpfs mounts
4. **User namespaces**: Solved with subuid/subgid configuration and keep-id

## Current Limitations

- **Single node only**: Multi-node clusters not yet implemented
- **Linux only**: Requires Linux kernel features
- **Cgroup v2 required**: Older systems with cgroup v1 won't work
- **Systemd required**: For service management in containers
- **Resource overhead**: Running systemd + CRI-O + k8s has overhead

## Future Enhancements

- Multi-node cluster support
- Load balancer for HA control planes
- Image pre-loading for faster pod starts
- Custom CNI plugin support
- Configuration file support (kipod.yaml)
- Support for multiple Kubernetes versions
- Cluster upgrade capabilities
- Registry mirror configuration

## Contributing

The codebase is structured for easy extension:

- Add new CLI commands in `cmd/kipod/`
- Add cluster features in `pkg/cluster/`
- Modify node image in `images/base/Containerfile`
- Add validation checks in `pkg/system/validate.go`

## Resources

- [Podman rootless documentation](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md)
- [CRI-O documentation](https://cri-o.io/)
- [kind project](https://kind.sigs.k8s.io/) (inspiration)
- [fuse-overlayfs](https://github.com/containers/fuse-overlayfs)
- [Kubernetes documentation](https://kubernetes.io/docs/home/)

---

**kipod** brings the kind experience to rootless Podman environments!
