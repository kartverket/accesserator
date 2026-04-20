#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo "🤞  Creating namespace: tokenx"

# Attempt to create the namespace and capture both stdout and stderr
# NOTE: `set -e` would abort the script on a non-zero exit code here (e.g. AlreadyExists),
# so we temporarily disable it to handle the error explicitly.
set +e
output=$("${KUBECTL_BIN}" create namespace "tokenx" --context "$KUBECONTEXT" 2>&1)
exit_code=$?
set -e

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'tokenx' created successfully"
elif echo "$output" | grep -qiE "already exists|AlreadyExists"; then
    echo "✅  Namespace 'tokenx' already exists, continuing..."
else
    echo -e "❌  Error creating 'tokenx' namespace:"
    echo "$output"
    exit 1
fi

# Install tokendings
TOKENDINGS_MANIFESTS="$(cat <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tokendings-ingress-egress
  namespace: tokenx
spec:
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: tokenx
          podSelector:
            matchLabels:
              app: database
      ports:
        - port: 5432
          protocol: TCP
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: auth
          podSelector:
            matchLabels:
              app: mock-oauth2
      ports:
        - port: 8080
          protocol: TCP
  ingress:
    - ports:
        - port: 7456
          protocol: TCP
  podSelector:
    matchLabels:
      app: tokendings
  policyTypes:
    - Ingress
    - Egress
---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-jwker
  namespace: tokenx
spec:
  action: DENY
  rules:
    - from:
        - source:
            notPrincipals:
              - cluster.local/ns/tokenx/sa/jwker
      to:
        - operation:
            paths:
              - /registration/client
---
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: tokendings
  namespace: tokenx
spec:
  accessPolicy:
    outbound:
      external:
        - host: test.idporten.no
        - host: test.ansattporten.no
        - host: login.microsoftonline.com
  env:
    - name: DB_JDBC_URL
      value: jdbc:postgresql://database:5432/token-exchange?user=user&password=pwd
    - name: AUTH_ACCEPTED_AUDIENCE
      value: http://tokendings.tokenx:7456/registration/client
    - name: AUTH_CLIENT_ID
      value: dfb2cec9-3b6d-456b-a14f-649236247e3d
    - name: AUTH_CLIENT_JWKS
      value: |
        {"keys":[{"d":"EeEtuQrs5k_kRasM-tOWuTe_mEjXtGJsjfTZId0v8lZ43r-LasZHq07OVERiWLv6grlUVKkxQ45dRh4yMK3YHGsCJKapBuRXKNfiwNYq9IrzHR04k8ADe9SfLS3Bu1_ig15SFEytzxYVn2Dswh6mDF1dtbL8z5xwLmOJhdL0UloBleYRvThkG_oQR8DzURUucS8newhTDE6xO5O5uAPmkDAEdkekWf93UQKipPv-QGA_dFf7Z5xB90qX8mW5qyAUcnaajPw6FufuP_VrGhfuTMPsJ0Aw1JBfxrazZFWPwGRuFFUaxbRN-OS7GLTpN9zd3DfX4gDsMs4vpJT7kkk10Q","dp":"OVbxZhOnW5rV_l5eKke1MfM7WuKRjiEjd0eL4Q8fQtFE9zNVhac3MimJQSUv4teTNJGFic7f9TTIpY25I9dQcDiqdp6Kob-7gqeOC8eAGNGtPi6ogO80WIri7XZ-mW4hBMp6UaXqC9KC7AoW0zbQMmkNWDkXg8KfnquCu29EQvE","dq":"ZgpZZ-mBBeMYHLBq4rmQyQA8aqJGbcMWWNGyzzlgsusYFYg9pKYWLwFVZPoQjzwAD4QTIGIUow3TC5kUkEwP7IiwjyuQwpexM3csSeK6Ag0MctmECehbIKfYN8TyOPM_TjxMJejOMs7BHwY2jnZ7Iakhx5yLvV2mfA2EhGz_cbU","e":"AQAB","kid":"1234","kty":"RSA","n":"vtHS1ulZpqxYPJZQmZrdgUPfMT4AR8sYL1VfLHwZc7bNo4fm1VRS0XwaRFYyOBBk1SXTCc7ojM8aO10NOHVYUWxMpr8mcxjWlFnn3xq0D_Z7DAKHt2cjIG2EPRzloH76qfqUCqmIHULTRuIavsOBmbE7dhTC63KKoO10i6KqP6iDeuqEds7-PyaqjB4F8-kgL9ukdVfo9AmWEnIm0bUvnNAhdyozoyGcmdI1bbtGWOo0RSb1t94LgVvWrpLzQulEfgHZpoW-TbrLlKhEA311BORSs55RhY8xDyijHNZQ-aShpQkROI7VELHyS6p54339g0z-FKp9Uwa8KQ2uijPp7Q","p":"6zv2JRlLYswnDmmMqYLFnYRW9CgBa-Aq9Cx41qfJi8NLLlVL_ZN1wg8WE9Bj3la6cAbUEIthzVc40G12Tk49HK-91lGmzy1zf6gQ2QD5vUSi4FEhNp_rGWtgE2GLn_1Y1268Gpr1DPkjHqPalh3krk7vd3ENbTZ9wNC7ikew75E","q":"z6oj5v91e1q6erKnvrjCj_Fqdai88heklEaW16xespfoibBrEmau6kUd4ojbKgCE4ZP8gX4ANzisW6TFoyNafqQ7HKSux87euW9EpyfdwmpGGTc7k9NMwtoIJzduqkaL6IebWBzXhpHn5Sguhri-qeJoQa6TSfdFKraK5Pm-Hp0","qi":"XjFAhzFySRwuXHhVtGTwt0UEnEwkfLHW606S4UD5bylSMSdroKAtb2KEnWuXf9tkixCW35JZAfPqBaAeqYn3Wz9hScZgV3qqJm_aJt1wN7Aih7mG__4dSOqWOp95MMZQs9HVfUqyTtz8Vv9O9PDzZf60CSHVQfVvPCFkHukLlcM"}]}
    - name: ISSUER_URL
      value: http://tokendings.tokenx:7456
    - name: APPLICATION_PORT
      value: "7456"
    - name: TOKEN_EXPIRY_SECONDS
      value: "300"
    - name: SUBJECT_TOKEN_ISSUERS
      value: https://test.idporten.no/.well-known/openid-configuration,https://test.ansattporten.no/.well-known/openid-configuration,https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0/.well-known/openid-configuration,http://mock-oauth2.auth:8080/accesserator/.well-known/openid-configuration
  image: ghcr.io/nais/tokendings@sha256:4eed6c9155809ab785705550a3b72ddbd916383c2bbd159b3a048c20429f4bb8
  port: 7456
  replicas:
    max: 3
    min: 2
    targetCpuUtilization: 80
    targetMemoryUtilization: 90
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: database-ingress
  namespace: tokenx
spec:
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: tokenx
          podSelector:
            matchLabels:
              app: tokendings
      ports:
        - port: 5432
          protocol: TCP
  podSelector:
    matchLabels:
      app: database
  policyTypes:
    - Ingress
---
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: database
  namespace: tokenx
spec:
  image: postgres
  port: 5432
  replicas: 1
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

"${KUBECTL_BIN}" apply -f <(echo "$TOKENDINGS_MANIFESTS") --context "$KUBECONTEXT"
