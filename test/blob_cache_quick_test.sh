#!/bin/bash
set -euo pipefail

TEST_DIR="${TEST_DIR:-/tmp/podman-blob-cache-test}"
CACHE_DIR="${TEST_DIR}/blob-cache"
STORAGE_CONF="${TEST_DIR}/storage.conf"
TEST_IMAGE="${TEST_IMAGE:-docker.io/library/alpine:latest}"

cleanup() {
  if [ "${KEEP_CACHE:-}" != "1" ]; then
    if command -v podman &> /dev/null && [ -f "${STORAGE_CONF}" ]; then
      export CONTAINERS_STORAGE_CONF="${STORAGE_CONF}"
      podman system reset -f &>/dev/null || true
    fi
    rm -rf "${TEST_DIR}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "Setting up blob cache test..."
mkdir -p "${CACHE_DIR}" "$(dirname "${STORAGE_CONF}")"

cat > "${STORAGE_CONF}" <<EOF
[storage]
driver = "overlay"
runroot = "${TEST_DIR}/run"
graphroot = "${TEST_DIR}/storage"

[storage.options]
blob_cache_dir = "${CACHE_DIR}"
blob_cache_role = "writer"
EOF

export CONTAINERS_STORAGE_CONF="${STORAGE_CONF}"

echo "Cache directory: ${CACHE_DIR}"
echo "Test image: ${TEST_IMAGE}"
echo ""

# First pull
echo "=== First pull (populates cache) ==="
time podman --log-level=debug pull "${TEST_IMAGE}"

cache_count=$(find "${CACHE_DIR}" -maxdepth 1 -type f ! -name "*.meta" ! -name "*.tmp" 2>/dev/null | wc -l)
echo "Cache files: ${cache_count}"
echo ""

# Reset
echo "Podman reset..."
podman system reset --force
echo ""

# Second pull
echo "=== Second pull (uses cache) ==="
time podman --log-level=debug pull "${TEST_IMAGE}"

cache_count=$(find "${CACHE_DIR}" -maxdepth 1 -type f ! -name "*.meta" ! -name "*.tmp" 2>/dev/null | wc -l)
echo "Cache files: ${cache_count}"
echo ""

echo "Test complete! Cache at: ${CACHE_DIR}"
echo "Set KEEP_CACHE=1 to preserve cache"
