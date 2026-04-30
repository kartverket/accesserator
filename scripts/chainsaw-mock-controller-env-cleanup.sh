#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

ENV_FILE=hack/mock_controller/.env
if [ ! -f "$ENV_FILE" ]; then
  echo "⚠️  hack/mock_controller/.env not found. No need to cleanup mock-controller-env secret."
  exit 0
fi
echo "⏳ Restoring mock-controller-env secret from $ENV_FILE"
"${KUBECTL_BIN}" create secret generic mock-controller-env \
  --from-env-file="$ENV_FILE" \
  -n mock-controller-system \
  --dry-run=client -o yaml | kubectl apply -f - --context "$KUBECONTEXT"
kubectl rollout restart deployment/mock-controller -n mock-controller-system
kubectl rollout status deployment/mock-controller -n mock-controller-system --timeout=30s
echo "✅ mock-controller-env secret restored and mock-controller restarted"