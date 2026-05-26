# Open Policy Agent (OPA)

A `SecurityConfig` can be used to configure the capability of setting up a fine-grained authorization server with Open Policy Agent (OPA) and policies written in OPA policy language [rego](https://www.openpolicyagent.org/docs/policy-language).
This allows you to define and maintain a custom authorization scheme independently of your application code, and leverage the powerful policy evaluation capabilities of OPA to make authorization decisions for your applications.

## Configuring OPA with `SecurityConfig`
The OPA capability is configured with `SecurityConfig` through the `opa` field, where you can specify the URL to one or more
OPA bundles used for authorization and the port that the OPA server should listen on. 

To verify the provenance of an OPA bundle, configure spec.opa.bundleUrls[*].verification. This assumes the bundle was built 
and signed using GitHub Actions with keyless Cosign signing, and that the signature is stored as an OCI 1.1 referrer. 
When configured, the verification checks that the bundle was built in the expected GitHub repository, by the expected workflow, 
and from the expected Git reference.

> [!TIP]
> An OPA bundle is a collection of OPA policies and data that are compiled together into a single `tar.gz` file.
> The bundle can then be loaded into an OPA server, which will evaluate the policies and data in the bundle to make authorization decisions.
> You can read more about OPA bundles in the [OPA documentation](https://www.openpolicyagent.org/docs/management-bundles).

The following `SecurityConfig` configures the OPA capability for a Skiperator application called `app`.
It specifies the URL to an OPA bundle containing policies and data for authorization.

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config
  namespace: opa-example
spec:
  opa:
    enabled: true
    bundleUrls:
      - name: authz-bundle
        url: ghcr.io/kartverket/accesserator/opa-bundle:setup-cosign-verification
        verification:
          source:
            repository: kartverket/accesserator
            workflow: .github/workflows/build-and-push-opa-bundle.yml
            ref: refs/pull/73
  applicationRef: app
```

> [!TIP]
> When referencing bundles from a container registry, the name field is required and must be unique for each bundle.
> The name is used by accesserator to identify the bundle and configure the OPA server accordingly.
> This way, you can update the bundle by simply pushing a new version to the container registry and
> updating the `SecurityConfig` with the new URL, without having to change your application code or redeploy your application.

## Perform authorization checks with `opa`
The above `SecurityConfig` will configure the OPA capability for the Skiperator application `app`, but to actually perform authorization checks with OPA, you need to make calls to the OPA server API from your application code.
The OPA server API can be made available to your application as a sidecar container in the same pod by configuring the pod annotation `accesserator.kartverket.no/services: "opa"`.
This will make accesserator inject the OPA server as a sidecar container in the application pod and configure it according to the `SecurityConfig` referencing the application.

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
      accesserator.kartverket.no/services: "opa"
```

The result is a pod where the main app container can call the API of the `opa` sidecar to perform authorization checks based on the policies and data in the OPA bundle specified in the `SecurityConfig`.

### Example: Perform authorization checks with `opa`
In this example, we will use the OPA bundle built from the folder [.opa](../../.opa/authz.rego), which is a simple policy which allows access (i.e. returns `{"result":true}`) if `input.password` is `password123`.

Check if a local cluster with all the necessary dependencies is configured and
that accesserator is running on your machine or as a deployment in the local cluster.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
make -C "$REPO_ROOT" ensurelocal ensurerunningordeployed
```

Apply the example resources, which include a Skiperator application with the `opa` sidecar injected
and a `SecurityConfig` that configures the OPA capability.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
"$KUBECTL" apply -f "$REPO_ROOT/examples/opa/manifests.yaml"
printf "Waiting for app pod to be ready"
until "$KUBECTL" -n opa-example get pods -l app=app 2>/dev/null | grep -q app; do
  printf "."; sleep 1
done
"$KUBECTL" wait --for=condition=Ready pod -l app=app -n opa-example --timeout=60s && printf " ✅\n"
```

Perform an authorization check that will **not** allow access (i.e. return `"false"`) by calling the API of the `opa` sidecar.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
OPA_URL=$("$KUBECTL" exec -n opa-example deploy/app -c app -- printenv OPA_URL)
OPA_RESPONSE=$("$KUBECTL" exec -n opa-example deploy/app -c app -- \
  curl -sX POST "$OPA_URL/v1/data/authz/allow" \
    -H "Content-Type: application/json" \
    -d "{\"input\":{\"password\":\"wrongpassword\"}}")
echo "OPA response:"
echo "$OPA_RESPONSE"
echo
```

Perform an authorization check that will allow access (i.e. return `"true"`) by calling the API of the `opa` sidecar.
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
KUBECTL="$REPO_ROOT/bin/kubectl"
make -C "$REPO_ROOT" kubectl
OPA_URL=$("$KUBECTL" exec -n opa-example deploy/app -c app -- printenv OPA_URL)
OPA_RESPONSE=$("$KUBECTL" exec -n opa-example deploy/app -c app -- \
  curl -sX POST "$OPA_URL/v1/data/authz/allow" \
    -H "Content-Type: application/json" \
    -d "{\"input\":{\"password\":\"accesserator\"}}")
echo "OPA response:"
echo "$OPA_RESPONSE"
echo
```