#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo "🤞  Setting up database for tokendings"

DB_MANIFESTS="$(cat <<EOF
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: database
  namespace: tokenx-api
spec:
  image: postgres
  port: 5432
  replicas: 1
  accessPolicy:
    inbound:
      rules:
        - application: tokendings
  env:
    - name: POSTGRES_USER
      value: user
    - name: POSTGRES_PASSWORD
      value: pwd
    - name: POSTGRES_DB
      value: token-exchange
    - name: PGDATA
      value: /tmp/data
  appProtocol: tcp
  filesFrom:
    - emptyDir: postgresql
      mountPath: /var/run/postgresql
EOF
)"

"${KUBECTL_BIN}" apply -f <(echo "$DB_MANIFESTS") --context "$KUBECONTEXT"
