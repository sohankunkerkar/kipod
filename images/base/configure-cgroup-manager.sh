#!/bin/bash
# Configure CRI-O and kubelet cgroup manager based on environment
# This script runs at container startup to configure the cgroup manager

set -e

# Default to cgroupfs for rootless compatibility
CGROUP_MANAGER="${KIPOD_CGROUP_MANAGER:-cgroupfs}"

echo "Configuring cgroup manager: ${CGROUP_MANAGER}"



write_crio_config() {
    local manager=$1
    local conmon_cgroup=$2
    
    cat > /etc/crio/crio.conf.d/10-cgroup-manager.conf <<EOF
[crio.runtime]
  cgroup_manager = "${manager}"
  conmon_cgroup = "${conmon_cgroup}"
  conmon_env = [
    "DBUS_SESSION_BUS_ADDRESS=unix:path=/var/run/dbus/system_bus_socket",
    "XDG_RUNTIME_DIR=/run/user/0"
  ]
EOF
}

write_kubelet_config() {
    local driver=$1
    
    cat > /etc/sysconfig/kubelet <<EOF
KUBELET_EXTRA_ARGS=--container-runtime-endpoint=unix:///var/run/crio/crio.sock --cgroup-driver=${driver} --fail-swap-on=false --feature-gates=KubeletInUserNamespace=true
EOF
}

if [ "$CGROUP_MANAGER" = "systemd" ]; then
    echo "Using systemd cgroup manager"

    # Configure CRI-O for systemd
    # Use conmon_cgroup = "system.slice" to use system-level systemd instead of systemctl --user
    write_crio_config "systemd" "system.slice"
    write_kubelet_config "systemd"
    
    # Force CRI-O to use system bus (requires our patch)
    echo "CRIO_FORCE_SYSTEM_BUS=true" >> /etc/sysconfig/crio
    
    echo "Configured for systemd cgroup manager"
elif [ "$CGROUP_MANAGER" = "cgroupfs" ]; then
    # Configure CRI-O for cgroupfs
    write_crio_config "cgroupfs" "pod"
    write_kubelet_config "cgroupfs"
    
    echo "Configured for cgroupfs cgroup manager"
else
    echo "Unknown cgroup manager: $CGROUP_MANAGER"
    exit 1
fi
