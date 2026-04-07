#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo "🤞  Creating namespace: mock-controller-system"

# Attempt to create the namespace and capture both stdout and stderr
# NOTE: `set -e` would abort the script on a non-zero exit code here (e.g. AlreadyExists),
# so we temporarily disable it to handle the error explicitly.
set +e
output=$("${KUBECTL_BIN}" create namespace "mock-controller-system" --context "$KUBECONTEXT" 2>&1)
exit_code=$?
set -e

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'mock-controller-system' created successfully"
elif echo "$output" | grep -qiE "already exists|AlreadyExists"; then
    echo "✅  Namespace 'mock-controller-system' already exists, continuing..."
else
    echo -e "❌  Error creating 'mock-controller-system' namespace:"
    echo "$output"
    exit 1
fi