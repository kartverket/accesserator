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
