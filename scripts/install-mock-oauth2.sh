#!/bin/bash

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
MOCK_OAUTH2_SERVER_VERSION=${MOCK_OAUTH2_SERVER_VERSION:-"2.2.1"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo -e "🤞 Retrieving content from mock-oauth2-config.json..."
JSON_CONTENT=$(cat "$MOCK_OAUTH2_CONFIG")
if [[ -z "$JSON_CONTENT" ]]; then
  echo "❌ Error: mock-oauth2-config.json is empty or not found at path: $MOCK_OAUTH2_CONFIG"
  exit 1
fi
echo -e "✅  Successfully retrieved content from mock-oauth2-config.json"

DEPLOYMENT="$(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: auth
---
# ConfigMap containing JSON config for mock-oauth2-server
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-oauth2-config
  namespace: auth
data:
  JSON_CONFIG: |
$(echo "$JSON_CONTENT" | sed 's/^/      /')
---
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: mock-oauth2
  namespace: auth
spec:
  image: ghcr.io/navikt/mock-oauth2-server:${MOCK_OAUTH2_SERVER_VERSION}
  port: 8080
  replicas: 1
  envFrom:
    - configMap: mock-oauth2-config
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow
  namespace: auth
spec:
  ingress:
  - ports:
    - port: 8080
      protocol: TCP
  podSelector:
    matchLabels:
      app: mock-oauth2
  policyTypes:
  - Ingress
EOF
)"

"${KUBECTL_BIN}" apply -f <(echo "$DEPLOYMENT") --context "$KUBECONTEXT"

while true; do
  SUMMARY_STATUS=$("${KUBECTL_BIN}" get application.skiperator.kartverket.no/mock-oauth2 -n auth -o jsonpath='{.status.summary.status}')

  if [[ "$SUMMARY_STATUS" == "Synced" ]]; then
    echo "✅ Application summary status is Synced."
    break
  fi

  sleep 1
  ELAPSED=$((ELAPSED + 1))
  if [[ "$ELAPSED" -ge 30 ]]; then
    echo "❌ Timeout: Application did not reach 'Synced' status in time."
    exit 1
  fi
done

"${KUBECTL_BIN}" wait --for=condition=InternalRulesValid=True application.skiperator.kartverket.no/mock-oauth2 -n auth --timeout=30s || (echo -e "❌  Error: accessPolicies for 'mock-oauth2' remain in InvalidConfig state." && exit 1)

POD_TIMEOUT=60
POD_ELAPSED=0
echo "⏳ Waiting for mock-oauth2 pod to become Ready (timeout: ${POD_TIMEOUT}s)..."

while true; do
  if [[ "$POD_ELAPSED" -gt 0 ]]; then
    sleep 5
    echo "  ... still waiting (${POD_ELAPSED}s elapsed, phase=${PHASE:-unknown}, reason=${REASON:-none})"
  fi

  if [[ "$POD_ELAPSED" -ge "$POD_TIMEOUT" ]]; then
    echo "❌ Timeout: mock-oauth2 pod did not become Ready within ${POD_TIMEOUT}s."
    [[ -n "$POD" ]] && kubectl describe pod "$POD" -n auth --context "$KUBECONTEXT"
    exit 1
  fi

  POD_ELAPSED=$((POD_ELAPSED + 5))

  POD=""
  PHASE=""
  REASON=""
  READY=""

  IFS=$'\t' read -r POD PHASE <<< "$(kubectl get pod -n auth -l app=mock-oauth2 --context "$KUBECONTEXT" \
    -o jsonpath='{.items[0].metadata.name}{"\t"}{.items[0].status.phase}' 2>/dev/null)"

  if [[ -n "$POD" ]]; then
    REASON=$(kubectl get pod "$POD" -n auth --context "$KUBECONTEXT" \
      -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null)
    READY=$(kubectl get pod "$POD" -n auth --context "$KUBECONTEXT" \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)

    if [[ "$REASON" =~ ^(CrashLoopBackOff|ImagePullBackOff|ErrImagePull|OOMKilled|Error)$ ]]; then
      echo "❌ Pod entered error state: $REASON"
      kubectl describe pod "$POD" -n auth --context "$KUBECONTEXT"
      kubectl logs "$POD" -n auth --context "$KUBECONTEXT" --tail=50 2>/dev/null || true
      exit 1
    fi

    if [[ "$PHASE" == "Running" && "$READY" == "True" ]]; then
      echo "✅ mock-oauth2 pod is Ready."
      break
    fi
  fi
done
