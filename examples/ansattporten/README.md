# Ansattporten

A `SecurityConfig` can be used to configure validation of [Ansattporten](https://docs.digdir.no/docs/Ansattporten/)
tokens. Unlike Maskinporten and Entra ID, Ansattporten support is **validation-only**: no client is registered. When
enabled, Accesserator creates an Istio `ServiceEntry` allowing egress to Ansattporten and configures the `texas`
sidecar to validate the `aud` (audience) claim of incoming Ansattporten tokens.

## Configuring Ansattporten with `SecurityConfig`

Ansattporten is enabled with a single `enabled` flag and exactly one allowed audience. The audience is the value the
`texas` sidecar validates incoming tokens' `aud` claim against (typically your application's Ansattporten client ID).

### 1. Specifying the audience directly

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  ansattporten:
    enabled: true
    allowedAudience:
      value: my-ansattporten-client-id
  applicationRef: app
```

### 2. Sourcing the audience from a ConfigMap

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  ansattporten:
    enabled: true
    allowedAudience:
      valueFrom:
        configMapKeyRef:
          name: my-config
          key: ansattporten-audience
  applicationRef: app
```

### 3. Sourcing the audience from a Secret

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
spec:
  ansattporten:
    enabled: true
    allowedAudience:
      valueFrom:
        secretKeyRef:
          name: my-secret
          key: ansattporten-audience
  applicationRef: app
```

> **Note:** Ansattporten tokens are validated against a single audience

## Validating Ansattporten tokens with `texas`

After configuring Ansattporten with `SecurityConfig`, the `texas` sidecar exposes a token introspection endpoint that
validates the signature, issuer and audience of an Ansattporten token. The sidecar is injected by annotating the
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

The main app container can then call the `texas` introspection API to validate an incoming Ansattporten token:

```bash
curl -sX POST "$TEXAS_URL/api/v1/introspect" \
  -H "Content-Type: application/json" \
  -d "{\"identity_provider\":\"ansattporten\",\"token\":\"<incoming-ansattporten-token>\"}"
```
