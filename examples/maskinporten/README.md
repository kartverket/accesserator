# Maskinporten

A `SecurityConfig` can be used to configure the capability of obtaining access tokens from Maskinporten, 
acting as an [API consumer](https://docs.digdir.no/docs/Maskinporten/maskinporten_guide_apikonsument). 

## Configuring Maskinporten with `SecurityConfig`
The maskinporten capability is configured with `SecurityConfig` in one of three ways; specifying the Maskinporten configuration directly in the `SecurityConfig` manifest, 
referencing an existing `MaskinportenClient` resource or referencing a Kubernetes Secret containing the necessary Maskinporten integration secrets.

### 1. Specifying the Maskinporten configuration directly in the `SecurityConfig` manifest
The following `SecurityConfig` configures the Maskinporten capability for a Skiperator application called `app` by specifying the necessary configuration directly in the manifest. 
```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  maskinporten:
    enabled: true
    client:
      clientName: app
      scopes:
        consumes:
          - name: demo311834282:write
          - name: demo311834282:read
  applicationRef: app
```

### 2. Referencing an existing `MaskinportenClient` resource
If you have already created a `MaskinportenClient` resource with the necessary configuration and want to reuse it, you can reference it in the `SecurityConfig` manifest as shown below. 
```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  maskinporten:
    enabled: true
    clientRef:
      name: existing-maskinporten-client
  applicationRef: app
```

### 3. Refer a Kubernetes Secret containing the Maskinporten integration secrets
If you have the necessary Maskinporten integration secrets stored in a Kubernetes Secret, you can reference it in the `SecurityConfig` manifest as shown below. 
```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  maskinporten:
    enabled: true
    secretRef:
      clientID:
        key: CLIENT_ID_KEY
        name: existing-maskinporten-secret
      clientJWK:
        key: CLIENT_JWK_KEY
        name: existing-maskinporten-secret
  applicationRef: app
```

## Obtaining access tokens from Maskinporten with `texas`
After configuring the Maskinporten capability with `SecurityConfig`, you can use the `texas` sidecar to obtain access tokens from Maskinporten.
The following manifest snippet shows how to configure a Skiperator application to have the `texas` sidecar injected.
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

The result is a pod where the main app container can call the API of the `texas` sidecar to obtain an access token from maskinporten.

### Example: Obtain maskinporten access token with `texas`
In this example, we use the CRD `MaskinportenClient` to configure the Maskinporten integration and reference it in the `SecurityConfig`. 
This is done by the Kubernetes operator [digdirator](https://github.com/nais/digdirator). Since digdirator requires an actual organisation 

Check if a local cluster with all the necessary dependencies is configured,
that accesserator is running on your machine or as a deployment in the local cluster, and that a local maskinporten 
mock controller is running on your machine or as a deployment in the local cluster. The mock controller watches `MaskinportenClient` resources and creates the necessary secrets for requesting maskinporten tokens. 
You can read more about how the mock controller works and how to run it in the [hack/mock_controller/README.md](../../hack/mock_controller/README.md) file.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
make -C "$REPO_ROOT" ensurelocal ensurerunningordeployed ensuremockcontrollerdeployed
```

Apply the example resources, which include a Skiperator application with the `texas` sidecar injected 
and a `SecurityConfig` that configures the maskinporten capability. 
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
"$KUBECTL" apply -f "$REPO_ROOT/examples/maskinporten/manifests.yaml"
printf "Waiting for app pod to be ready"
until "$KUBECTL" -n maskinporten-example get pods -l app=app 2>/dev/null | grep -q app; do
  printf "."; sleep 1
done
"$KUBECTL" wait --for=condition=Ready pod -l app=app -n maskinporten-example --timeout=60s && printf " ✅\n"
```

Retrieve a maskinporten access token from the `texas` sidecar by executing the following command. 
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
TEXAS_URL=$("$KUBECTL" exec -n maskinporten-example deploy/app -c app -- printenv TEXAS_URL)
TEXAS_RESPONSE=$("$KUBECTL" exec -n maskinporten-example deploy/app -c app -- \
  curl -sX POST "$TEXAS_URL/api/v1/token" \
    -H "Content-Type: application/json" \
    -d "{\"identity_provider\":\"maskinporten\",\"target\":\"demo311834282:write\"}")
echo "Texas token exchange response:"
echo "$TEXAS_RESPONSE"
echo
echo "Access token payload parsed:"
echo "$TEXAS_RESPONSE" | jq -r '.access_token' | jq -R 'split(".") | .[1] | @base64d | fromjson'
```