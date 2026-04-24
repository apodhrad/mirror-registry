#!/bin/bash
set -e

SEED_IMAGES_FILE="${1:-seed-images.txt}"

if [ ! -f "$SEED_IMAGES_FILE" ]; then
    echo "Usage: $0 [seed-images.txt]"
    echo "Error: seed images file not found: $SEED_IMAGES_FILE"
    exit 1
fi

if ! command -v skopeo &>/dev/null; then
    echo "Error: skopeo is not installed"
    exit 1
fi

authfile_opt=""
if [ -n "$REGISTRY_AUTH_FILE" ]; then
    authfile_opt="--authfile $REGISTRY_AUTH_FILE"
fi

tmpdir=$(mktemp -d)
trap "rm -rf $tmpdir" EXIT

mkdir -p "$tmpdir/seed-images"

while IFS= read -r image; do
    [[ -z "$image" || "$image" =~ ^[[:space:]]*# ]] && continue
    filename=$(echo "$image" | sed 's|[/:@]|_|g').tar
    echo "Pulling $image..."
    skopeo copy $authfile_opt --all --remove-signatures "docker://${image}" "oci-archive:${tmpdir}/seed-images/${filename}" || \
        { echo "ERROR: failed to pull ${image}"; exit 1; }
done < "$SEED_IMAGES_FILE"

cp "$SEED_IMAGES_FILE" "$tmpdir/seed-images/seed-images.txt"

echo "Creating seed-images.tar..."
tar -cvf seed-images.tar -C "$tmpdir/seed-images" .
echo "Done: seed-images.tar"
