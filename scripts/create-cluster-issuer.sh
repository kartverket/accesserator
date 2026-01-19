set -euo pipefail

kubectl apply --context "$KUBECONTEXT" -f <(cat <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: cluster-issuer
spec:
  selfSigned: {}
EOF
)