#!/bin/bash
# Configure CRI-O and kubelet cgroup manager based on environment
# This script runs at container startup to configure the cgroup manager

set -e

# Default to cgroupfs for rootless compatibility
CGROUP_MANAGER="${KIPOD_CGROUP_MANAGER:-cgroupfs}"

echo "Configuring cgroup manager: ${CGROUP_MANAGER}"

# Always set _CRIO_ROOTLESS=1 so CRI-O skips OOM score adjustments
# This triggers makeOCIConfigurationRootless() in CRI-O which is needed for both modes
echo "_CRIO_ROOTLESS=1" >> /etc/sysconfig/crio



write_crio_config() {
    local manager=$1
    local conmon_cgroup=$2
    
    cat > /etc/crio/crio.conf.d/10-cgroup-manager.conf <<EOF
[crio.runtime]
  cgroup_manager = "${manager}"
  conmon_cgroup = "${conmon_cgroup}"
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
    # Use conmon_cgroup = "system.slice" to use system-level systemd
    write_crio_config "systemd" "system.slice"
    write_kubelet_config "systemd"
elif [ "$CGROUP_MANAGER" = "cgroupfs" ]; then
    echo "Using cgroupfs cgroup manager"
    write_crio_config "cgroupfs" "pod"
    write_kubelet_config "cgroupfs"
else
    echo "Unknown cgroup manager: $CGROUP_MANAGER"
    exit 1
fi

echo "Configured for ${CGROUP_MANAGER} cgroup manager"
