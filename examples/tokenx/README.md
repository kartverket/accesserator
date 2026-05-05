# Token Exchange

A `SecurityConfig` can be used to configure the capability of performing token exchange. 
Token exchange is where an application can exchange an access token issued for one audience, with an access token that is destined for another audience, while preserving the user context. 
This is useful in scenarios where you have multiple applications that need to call each other on behalf of the end user, and you want to avoid the end user having to obtain and manage multiple access tokens for different applications. 
It is also useful in scenarios where you want to limit the scope of the access token that is used to call a downstream application, by exchanging it for a token that has a more limited scope.

## Configuring token exchange with `SecurityConfig`
The following `SecurityConfig` configures the token exchange capability for a Skiperator application called `app`.

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  tokenx:
    enabled: true
    accessPolicy:
      inheritInboundRules: true
  applicationRef: app
```

## Exchange tokens with `texas`
You can use `texas` to perform the token exchange. The following manifest snippet shows how to configure a Skiperator application to have the `texas` sidecar injected. 
The sidecar will be configured given the `SecurityConfig` referencing the application.

```yaml
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: app
spec:
  image: image
  port: 8080
  podSettings:
    annotations:
      accesserator.kartverket.no/services: "texas"
```

The result is a pod where the main app container can call the API of the `texas` sidecar to perform token exchange.

### Example: Exchange tokens with `texas`
Check if a local cluster with all the necessary dependencies is configured, 
that accesserator is running on your machine or as a deployment in the local cluster and that an ingress is set up to access the mock-oauth2 server.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
make -C "$REPO_ROOT" ensurelocal ensurerunningordeployed mock-oauth2-ingress
```

Apply the example resources, which include two Skiperator applications where `app` uses the `texas` sidecar to exchange tokens for accessing `another-app` on behalf of the end user.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
"$KUBECTL" apply -f "$REPO_ROOT/examples/tokenx/manifests.yaml"
printf "Waiting for app pod to be ready"
until "$KUBECTL" -n tokenx-example get pods -l app=app 2>/dev/null | grep -q app; do
  printf "."; sleep 1
done
"$KUBECTL" wait --for=condition=Ready pod -l app=app -n tokenx-example --timeout=60s && printf " ✅\n"
```

Issue a token from the mock-oauth2 server. As `app`, exchange it for a token that can be used to access `another-app` on behalf of the end user.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
MOCK_TOKEN="$(make -C "$REPO_ROOT" mock-token)"
echo "Payload of mock token issued for user access:"
echo "$MOCK_TOKEN" | jq -R 'split(".") | .[1] | @base64d | fromjson'
TEXAS_URL=$("$KUBECTL" exec -n tokenx-example deploy/app -c app -- printenv TEXAS_URL)
TEXAS_RESPONSE=$("$KUBECTL" exec -n tokenx-example deploy/app -c app -- \
  curl -sX POST "$TEXAS_URL/api/v1/token/exchange" \
    -H "Content-Type: application/json" \
    -d "{\"identity_provider\":\"tokenx\",\"target\":\"kind-accesserator:tokenx-example:another-app\",\"user_token\":\"$MOCK_TOKEN\"}")
echo "Texas token exchange response:"
echo "$TEXAS_RESPONSE"
echo
echo "Access token payload parsed:"
echo "$TEXAS_RESPONSE" | jq -r '.access_token' | jq -R 'split(".") | .[1] | @base64d | fromjson'
```