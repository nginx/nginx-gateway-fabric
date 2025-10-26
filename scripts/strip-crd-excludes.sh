#!/bin/bash

# This script removes the <gateway:util:excludeFromCRD> sections from generated CRD files
# These sections are implementation notes from Gateway API v1.4.0+ that should not appear
# in the final CRD definitions

set -e

CRD_DIR="${1:-config/crd/bases}"

echo "Stripping <gateway:util:excludeFromCRD> sections from CRD files in ${CRD_DIR}"

for file in "${CRD_DIR}"/*.yaml; do
    if [ -f "$file" ]; then
        # Use awk to remove lines between the exclude tags
        awk '
        /<gateway:util:excludeFromCRD>/ { skip=1; next }
        /<\/gateway:util:excludeFromCRD>/ { skip=0; next }
        !skip { print }
        ' "$file" >"$file.tmp" && mv "$file.tmp" "$file"

        echo "  Processed: $file"
    fi
done

echo "Done!"
