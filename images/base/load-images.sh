#!/bin/bash
# Load pre-downloaded Kubernetes images into CRI-O
# These images were downloaded during image build to avoid pull issues

IMAGE_DIR="/kind/images"

if [ ! -d "$IMAGE_DIR" ]; then
    echo "No pre-downloaded images found at $IMAGE_DIR"
    exit 0
fi

# Wait for CRI-O socket to be available
for i in {1..30}; do
    if [ -S /var/run/crio/crio.sock ]; then
        break
    fi
    echo "Waiting for CRI-O socket..."
    sleep 1
done

if [ ! -S /var/run/crio/crio.sock ]; then
    echo "CRI-O socket not available, skipping image load"
    exit 1
fi

echo "Loading pre-downloaded images into CRI-O..."

for tarball in "$IMAGE_DIR"/*.tar; do
    if [ -f "$tarball" ]; then
        echo "Loading: $(basename "$tarball")"
        # Use ctr or crictl to import the image
        # crictl doesn't support docker-archive, so we use skopeo to copy to containers-storage
        image_name=$(tar -xOf "$tarball" manifest.json 2>/dev/null | jq -r '.[0].RepoTags[0]' 2>/dev/null || echo "")
        if [ -n "$image_name" ] && [ "$image_name" != "null" ]; then
            skopeo copy "docker-archive:$tarball" "containers-storage:$image_name" 2>/dev/null || \
                echo "Warning: Failed to load $tarball"
        fi
    fi
done

echo "Image loading complete"



