#!/bin/bash
# crun wrapper to handle oomScoreAdj for rootless containers
# This script intercepts crun calls and removes oomScoreAdj to avoid permission errors

# Path to the real crun binary
REAL_CRUN="/usr/bin/crun.real"

# Debug log file
DEBUG_LOG="/tmp/crun-wrapper.log"

# Log wrapper invocation
echo "[$(date)] Wrapper called with args: $*" >> "$DEBUG_LOG" 2>&1

# Parse arguments to find the bundle path
bundle_path=""
for ((i=1; i<=$#; i++)); do
    arg="${!i}"
    if [ "$arg" = "--bundle" ] || [ "$arg" = "-b" ]; then
        # Next argument is the bundle path
        next_i=$((i+1))
        bundle_path="${!next_i}"
        break
    elif [[ "$arg" == --bundle=* ]]; then
        # Handle --bundle=<path> format
        bundle_path="${arg#--bundle=}"
        break
    fi
done

echo "[$(date)] Bundle path: $bundle_path" >> "$DEBUG_LOG" 2>&1

# If we found a bundle path and config.json exists, modify oomScoreAdj
if [ -n "$bundle_path" ] && [ -f "$bundle_path/config.json" ]; then
    echo "[$(date)] Found config.json at $bundle_path/config.json" >> "$DEBUG_LOG" 2>&1

    # Get current process's oom_score_adj (max value we can set)
    current_oom=$(cat /proc/self/oom_score_adj 2>/dev/null || echo "0")
    echo "[$(date)] Current OOM score: $current_oom" >> "$DEBUG_LOG" 2>&1

    # Remove oomScoreAdj entirely to avoid permission issues in rootless/nested containers
    # If jq is not available, just run crun normally
    if command -v jq >/dev/null 2>&1; then
        config_file="$bundle_path/config.json"
        temp_file="$config_file.tmp"

        # Check if oomScoreAdj exists in the config
        if jq -e '.process.oomScoreAdj' "$config_file" >/dev/null 2>&1; then
            orig_value=$(jq -r '.process.oomScoreAdj' "$config_file")
            echo "[$(date)] Found oomScoreAdj=$orig_value, removing it" >> "$DEBUG_LOG" 2>&1
            # Remove the oomScoreAdj field entirely
            if jq 'del(.process.oomScoreAdj)' "$config_file" > "$temp_file" && mv "$temp_file" "$config_file"; then
                echo "[$(date)] Successfully removed oomScoreAdj from config.json" >> "$DEBUG_LOG" 2>&1
            else
                echo "[$(date)] Failed to remove oomScoreAdj" >> "$DEBUG_LOG" 2>&1
            fi
        else
            echo "[$(date)] No oomScoreAdj found in config.json" >> "$DEBUG_LOG" 2>&1
        fi
    else
        echo "[$(date)] jq not available" >> "$DEBUG_LOG" 2>&1
    fi
else
    echo "[$(date)] No bundle path or config.json not found" >> "$DEBUG_LOG" 2>&1
fi

echo "[$(date)] Executing real crun" >> "$DEBUG_LOG" 2>&1

# Execute the real crun with all original arguments
exec "$REAL_CRUN" "$@"
