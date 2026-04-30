#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
CHAINSAW_BIN="${CHAINSAW_BIN:-./bin/chainsaw}"

# Run cleanup by restoring mock-controller-env if any of the tests fails
trap './scripts/chainsaw-mock-controller-env-cleanup.sh' EXIT

"${CHAINSAW_BIN}" test --kube-context "$KUBECONTEXT" --config test/chainsaw/config.yaml --test-dir test/chainsaw/securityconfig && echo "✅ Tests succeeded" || (echo "❌ Tests failed" && exit 1)