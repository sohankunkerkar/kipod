#!/bin/bash
set -e

# Entrypoint script for kipod node container
# Handles initialization before systemd takes over

echo "Kipod node container starting..."

# Load kernel modules if possible (might fail in rootless, that's okay)
modprobe overlay 2>/dev/null || echo "Warning: Could not load overlay module (may already be loaded)"
modprobe br_netfilter 2>/dev/null || echo "Warning: Could not load br_netfilter module (may already be loaded)"

# Apply sysctl settings (might fail in rootless, that's okay)
sysctl --system 2>/dev/null || echo "Warning: Could not apply sysctl settings (may require host configuration)"

# Ensure cgroup hierarchy is set up
if [ -d /sys/fs/cgroup ]; then
    # Mount cgroup v2 if needed
    if ! mountpoint -q /sys/fs/cgroup; then
        mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null || echo "Warning: Could not mount cgroup2"
    fi
fi

# Create runtime directories
mkdir -p /var/run/crio
mkdir -p /var/lib/crio
mkdir -p /var/lib/kubelet

# Set proper permissions
chmod 755 /var/run/crio
chmod 755 /var/lib/crio
chmod 755 /var/lib/kubelet

echo "Kipod node initialized, starting systemd..."

# Execute the CMD (systemd)
exec "$@"
