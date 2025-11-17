#!/usr/bin/env python3
"""
Patch CRI-O's dbusmgr.go to support CRIO_FORCE_SYSTEM_BUS environment variable.
This allows forcing system bus usage even when running in a user namespace.
"""

import sys

def patch_dbusmgr(filepath):
    with open(filepath, 'r') as f:
        lines = f.readlines()
    
    # Find the import block and add "os"
    import_added = False
    for i, line in enumerate(lines):
        if line.strip() == 'import (' and not import_added:
            # Check if "os" is already imported
            has_os = any('"os"' in l for l in lines[i:i+10])
            if not has_os:
                # Add os import after the opening (
                lines.insert(i+1, '\t"os"\n')
                import_added = True
            break
    
    # Find and replace the panic check
    for i, line in enumerate(lines):
        if 'panic("can\'t have both root and rootless dbus")' in line:
            # Find the if condition before it
            if_idx = i - 1
            if 'if dbusInited && rootless != dbusRootless {' in lines[if_idx]:
                # Replace the if condition
                lines[if_idx] = '\tif dbusInited && rootless != dbusRootless && os.Getenv("CRIO_FORCE_SYSTEM_BUS") != "true" {\n'
            break

    # Find and replace the dbusRootless = rootless line
    for i, line in enumerate(lines):
        if line.strip() == 'dbusRootless = rootless':
            # Replace with conditional logic
            indent = '\t'
            lines[i] = f'{indent}// Allow forcing system bus usage via environment variable\n'
            lines.insert(i+1, f'{indent}// This is useful for nested containers that run full systemd\n')
            lines.insert(i+2, f'{indent}if os.Getenv("CRIO_FORCE_SYSTEM_BUS") == "true" {{\n')
            lines.insert(i+3, f'{indent}\tdbusRootless = false\n')
            lines.insert(i+4, f'{indent}}} else {{\n')
            lines.insert(i+5, f'{indent}\tdbusRootless = rootless\n')
            lines.insert(i+6, f'{indent}}}\n')
            break
    
    with open(filepath, 'w') as f:
        f.writelines(lines)
    
    print(f"Successfully patched {filepath}")

if __name__ == '__main__':
    if len(sys.argv) != 2:
        print("Usage: patch_dbusmgr.py <path_to_dbusmgr.go>")
        sys.exit(1)
    
    patch_dbusmgr(sys.argv[1])
