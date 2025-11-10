#!/bin/bash
set -euo pipefail

TEST_DIR="${TEST_DIR:-/tmp/podman-blob-cache-test}"
CACHE_DIR="${TEST_DIR}/blob-cache"
STORAGE_CONF="${TEST_DIR}/storage.conf"
TEST_IMAGE="${TEST_IMAGE:-quay.io/containers/podman@sha256:7dbeb8b7a8f83ef96bad4c4f04fbdecedc65d36a5c6953a1861eb94386f3c4ec}"

echo "Setting up blob cache test..."
sudo rm -rf "${TEST_DIR}"
mkdir -p "${CACHE_DIR}" "$(dirname "${STORAGE_CONF}")"

cat > "${STORAGE_CONF}" <<EOF
[storage]
driver = "overlay"
runroot = "${TEST_DIR}/run"
graphroot = "${TEST_DIR}/storage"

[storage.options]
blob_cache_dir = "${CACHE_DIR}"
EOF

export CONTAINERS_STORAGE_CONF="${STORAGE_CONF}"

# First pull
echo ""
echo "First pull (populates cache)"
start=$(date +%s)
output="$(podman --log-level=debug pull "${TEST_IMAGE}" 2>&1)"
pulltime=$(($(date +%s) - $start))
echo "$output" | grep "added blob" | awk -F 'msg=' '{print $2}'
echo "Pull took $pulltime seconds"

# Reset
echo ""
echo "Reset podman storage..."
podman system reset --force 2>&1 > /dev/null

# Make cache read-only
# a typical setup would user different users
echo "Cache set read-only..."
sudo chmod 755 $CACHE_DIR
sudo find "${CACHE_DIR}" -type f -exec chmod 444 {} \; # Set file permissions to 444 (read-only)
sudo find "${CACHE_DIR}" -type d -exec chmod 755 {} \; # Ensure directories remain 755 (readable and traversable)

# Inspect cache
echo ""
echo "Cache:"
ls -lh "${CACHE_DIR}"

# Second pull
echo ""
echo "Second pull (uses cache)"
start=$(date +%s)
output="$(podman --log-level=debug pull "${TEST_IMAGE}" 2>&1)"
pulltime=$(($(date +%s) - $start))
echo "$output" | grep "already" | awk -F 'msg=' '{print $2}'
echo "Pull took $pulltime seconds"
