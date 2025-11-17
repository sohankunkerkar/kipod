#!/bin/bash
# crun wrapper to handle oomScoreAdj for rootless containers
# This script intercepts crun calls and clamps oomScoreAdj to valid values

# Path to the real crun binary
REAL_CRUN="/usr/bin/crun.real"

# Parse arguments to find the bundle path
bundle_path=""
for ((i=1; i<=$#; i++)); do
    arg="${!i}"
    if [ "$arg" = "--bundle" ] || [ "$arg" = "-b" ]; then
        # Next argument is the bundle path
        next_i=$((i+1))
        bundle_path="${!next_i}"
        break
    fi
done

# If we found a bundle path and config.json exists, modify oomScoreAdj
if [ -n "$bundle_path" ] && [ -f "$bundle_path/config.json" ]; then
    # Get current process's oom_score_adj (max value we can set)
    current_oom=$(cat /proc/self/oom_score_adj 2>/dev/null || echo "0")

    # Use jq to clamp oomScoreAdj if it's lower than current value
    # If jq is not available, just run crun normally
    if command -v jq >/dev/null 2>&1; then
        config_file="$bundle_path/config.json"
        temp_file="$config_file.tmp"

        # Read the current oomScoreAdj value
        oom_adj=$(jq -r '.process.oomScoreAdj // empty' "$config_file" 2>/dev/null)

        if [ -n "$oom_adj" ] && [ "$oom_adj" != "null" ]; then
            # If requested oom_score_adj is lower than current, clamp it
            if [ "$oom_adj" -lt "$current_oom" ]; then
                jq ".process.oomScoreAdj = $current_oom" "$config_file" > "$temp_file" && \
                    mv "$temp_file" "$config_file"
            fi
        fi
    fi
fi

# Execute the real crun with all original arguments
exec "$REAL_CRUN" "$@"
