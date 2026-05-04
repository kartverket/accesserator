#!/bin/bash

# Ensure both arguments are provided
if [[ -z "$1" || -z "$2" ]]; then
  echo "❌ Error: Missing arguments."
  echo "Usage: $0 <app_name> <namespace>"
  exit 1
fi

APP_NAME="$1"
NAMESPACE="$2"

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo "Checking status for $APP_NAME in namespace $NAMESPACE..."

ELAPSED=0
while true; do
  SUMMARY_STATUS=$("${KUBECTL_BIN}" get application.skiperator.kartverket.no/"$APP_NAME" -n "$NAMESPACE" --context "$KUBECONTEXT" -o jsonpath='{.status.summary.status}' 2>/dev/null)

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

"${KUBECTL_BIN}" wait --for=condition=InternalRulesValid=True application.skiperator.kartverket.no/"$APP_NAME" -n "$NAMESPACE" --context "$KUBECONTEXT" --timeout=30s || (echo -e "❌ Error: accessPolicies for '$APP_NAME' remain in InvalidConfig state." && exit 1)

POD_TIMEOUT=60
POD_ELAPSED=0
echo "⏳ Waiting for $APP_NAME pod to become Ready (timeout: ${POD_TIMEOUT}s)..."

while true; do
  if [[ "$POD_ELAPSED" -gt 0 ]]; then
    sleep 5
    echo "  ... still waiting (${POD_ELAPSED}s elapsed, phase=${PHASE:-unknown}, reason=${REASON:-none})"
  fi

  if [[ "$POD_ELAPSED" -ge "$POD_TIMEOUT" ]]; then
    echo "❌ Timeout: $APP_NAME pod did not become Ready within ${POD_TIMEOUT}s."
    [[ -n "$POD" ]] && "${KUBECTL_BIN}" describe pod "$POD" -n "$NAMESPACE" --context "$KUBECONTEXT"
    exit 1
  fi

  POD_ELAPSED=$((POD_ELAPSED + 5))

  POD=""
  PHASE=""
  REASON=""
  READY=""

  # Using app=$APP_NAME label selector
  IFS=$'\t' read -r POD PHASE <<< "$("${KUBECTL_BIN}" get pod -n "$NAMESPACE" -l app="$APP_NAME" --context "$KUBECONTEXT" \
    -o jsonpath='{.items[0].metadata.name}{"\t"}{.items[0].status.phase}' 2>/dev/null)"

  if [[ -n "$POD" ]]; then
    REASON=$("${KUBECTL_BIN}" get pod "$POD" -n "$NAMESPACE" --context "$KUBECONTEXT" \
      -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null)
    READY=$("${KUBECTL_BIN}" get pod "$POD" -n "$NAMESPACE" --context "$KUBECONTEXT" \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)

    if [[ "$REASON" =~ ^(CrashLoopBackOff|ImagePullBackOff|ErrImagePull|OOMKilled|Error)$ ]]; then
      echo "❌ Pod entered error state: $REASON"
      "${KUBECTL_BIN}" describe pod "$POD" -n "$NAMESPACE" --context "$KUBECONTEXT"
      "${KUBECTL_BIN}" logs "$POD" -n "$NAMESPACE" --context "$KUBECONTEXT" --tail=50 2>/dev/null || true
      exit 1
    fi

    if [[ "$PHASE" == "Running" && "$READY" == "True" ]]; then
      echo "✅ $APP_NAME pod is Ready."
      break
    fi
  fi
done
