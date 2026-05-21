#!/bin/bash

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

MOCK_CONTROLLER_RESOURCES=(
  https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_maskinportenclients.yaml
  https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_azureadapplications.yaml
)

echo "🤞  Creating namespace: $namespace_name"

# Attempt to create the namespace and capture both stdout and stderr
output=$("${KUBECTL_BIN}" create namespace "mock-controller-system" 2>&1)
exit_code=$?

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'mock-controller-system' created successfully"
elif echo "$output" | grep -q "already exists"; then
    echo "✅  Namespace 'mock-controller-system' already exists, continuing..."
else
    echo -e "❌  Error creating 'mock-controller-system' namespace."
    exit 1
fi

# Install required mock-controller resources
for resource in "${MOCK_CONTROLLER_RESOURCES[@]}"; do
  "${KUBECTL_BIN}" apply --context "$KUBECONTEXT" -f "$resource"
done

# Install skiperator
MOCK_CONTROLLER_MANIFESTS="$(cat <<EOF
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: mock-controller
rules:
- apiGroups:
  - nais.io
  resources:
  - maskinportenclients/finalizers
  - azureadapplications/finalizers
  verbs:
  - update
- apiGroups:
  - nais.io
  resources:
  - maskinportenclients/status
  - azureadapplications/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - events.k8s.io
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - nais.io
  resources:
  - maskinportenclients
  - azureadapplications
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
---
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: "mock-controller-system"
  name: "mock-controller"
automountServiceAccountToken: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mock-controller
roleRef:
  apiGroup: "rbac.authorization.k8s.io"
  kind: "ClusterRole"
  name: "mock-controller"
subjects:
  - kind: "ServiceAccount"
    namespace: "mock-controller-system"
    name: "mock-controller"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "mock-controller"
  namespace: mock-controller-system
  labels:
    app: "mock-controller"
spec:
  selector:
    matchLabels:
      app: "mock-controller"
  replicas: 1
  template:
    metadata:
      labels:
        app: "mock-controller"
    spec:
      serviceAccountName: "mock-controller"
      automountServiceAccountToken: true
      containers:
        - name: "mock-controller"
          envFrom:
            - secretRef:
                name: mock-controller-env
          image: "mock-controller:latest"
          imagePullPolicy: Never
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            runAsUser: 65532
            runAsGroup: 65532
            runAsNonRoot: true
            privileged: false
            seccompProfile:
              type: "RuntimeDefault"
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
            limits:
              memory: 256Mi
          ports:
            - name: "probes"
              containerPort: 8083
          livenessProbe:
            httpGet:
              path: "/healthz"
              port: "probes"
          readinessProbe:
            httpGet:
              path: "/readyz"
              port: "probes"
EOF
)"

"${KUBECTL_BIN}" apply -f <(echo "$MOCK_CONTROLLER_MANIFESTS") --context "$KUBECONTEXT"
