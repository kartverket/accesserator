# ID-porten

A `SecurityConfig` can be used to configure validation of [ID-porten](https://docs.digdir.no/docs/idporten/) tokens.
Unlike Maskinporten and Entra ID, ID-porten support is **validation-only**: no client is registered. When enabled,
Accesserator creates an Istio `ServiceEntry` allowing egress to ID-porten and configures the `texas` sidecar to
validate the `aud` (audience) claim of incoming ID-porten tokens.

## Configuring ID-porten with `SecurityConfig`

ID-porten is enabled with a single `enabled` flag and exactly one allowed audience. The audience is the value the
`texas` sidecar validates incoming tokens' `aud` claim against (typically your application's ID-porten client ID).

### 1. Specifying the audience directly

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  idporten:
    enabled: true
    allowedAudience:
      value: my-idporten-client-id
  applicationRef: app
```

### 2. Sourcing the audience from a ConfigMap

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  idporten:
    enabled: true
    allowedAudience:
      valueFrom:
        configMapKeyRef:
          name: my-config
          key: idporten-audience
  applicationRef: app
```

### 3. Sourcing the audience from a Secret

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  idporten:
    enabled: true
    allowedAudience:
      valueFrom:
        secretKeyRef:
          name: my-secret
          key: idporten-audience
  applicationRef: app
```

> **Note:** ID-porten tokens are validated against a single audience, so exactly one `allowedAudience` entry must be
> specified when `idporten.enabled` is `true`. This is enforced by CRD validation at admission time.

## Validating ID-porten tokens with `texas`

After configuring ID-porten with `SecurityConfig`, the `texas` sidecar exposes a token introspection endpoint that
validates the signature, issuer and audience of an ID-porten token. The sidecar is injected by annotating the
Skiperator application with `accesserator.kartverket.no/services: "texas"`.

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

The main app container can then call the `texas` introspection API to validate an incoming ID-porten token:

```bash
curl -sX POST "$TEXAS_URL/api/v1/introspect" \
  -H "Content-Type: application/json" \
  -d "{\"identity_provider\":\"idporten\",\"token\":\"<incoming-idporten-token>\"}"
```
