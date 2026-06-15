# Agent Instructions for Accesserator

## Project Overview

Accesserator is a Kubernetes operator built with **Kubebuilder** that lets teams configure security capabilities for [Skiperator](https://github.com/kartverket/skiperator) applications through a single custom resource called `SecurityConfig`. It is part of Kartverket's **SKIP platform** (`skip.kartverket.no`) and is owned by team **tilgangsstyring** (access management).

The operator manages four independent feature groups — **TokenX**, **Maskinporten**, **Entra ID**, and **OPA** — each generating their own set of child Kubernetes resources. When requested, it injects two types of sidecar containers into application pods via admission webhooks:

- **[Texas](https://github.com/nais/texas)** (`texas`): A sidecar that provides an API for token-related operations (retrieval, exchange, introspection) for TokenX, Maskinporten, and Entra ID.
- **OPA** (`opa`): An Open Policy Agent sidecar for policy evaluation, fed with bundles fetched from OCI registries.

The sidecar injection is triggered by the pod annotation `accesserator.kartverket.no/services: texas,opa`.

## Domain Model

### Custom Resource: `SecurityConfig`

The single CRD `SecurityConfig` (`accesserator.kartverket.no/v1alpha`) is the only user-facing resource. It targets a Skiperator `Application` via `spec.applicationRef` and enables one or more security features through optional spec fields.

```
SecurityConfig
  ├─ spec.applicationRef          (required) — name of the target Skiperator Application
  ├─ spec.tokenx                  (optional) — TokenX / token exchange configuration
  ├─ spec.maskinporten            (optional) — Maskinporten API consumer configuration
  ├─ spec.entraid                 (optional) — Entra ID (Azure AD) configuration
  └─ spec.opa                     (optional) — Open Policy Agent bundle configuration
```

### Feature Groups and Generated Resources

Each enabled feature generates a specific set of child resources, all owned via `ownerReferences` on the child resources pointing to the `SecurityConfig`

#### TokenX

| Generated Resource | Naming Convention | Purpose |
|---|---|---|
| `Jwker` | `<applicationRef>` | Registers an OAuth2 client in Tokendings and creates a JWK secret used by the Texas sidecar |
| `NetworkPolicy` | `<securityConfigName>-tokendings-egress` | Allows egress traffic to the Tokendings token exchange server |

#### Maskinporten

| Generated Resource | Naming Convention | Purpose |
|---|---|---|
| `MaskinportenClient` | `<applicationRef>` | Registers a client at Digdir via nais/digdirator (only for `InlineClient` and `None` config types) |
| `Secret` | `<applicationRef>-maskinporten-<hash>` (inline) or `<securityConfigName>-maskinporten-<hash>` (ref/secretRef) | Holds Maskinporten credentials for the Texas sidecar |
| `ServiceEntry` | `<securityConfigName>-maskinporten-<hash>` | Allows egress to `maskinporten.no` / `test.maskinporten.no` through Istio |

#### Entra ID

| Generated Resource | Naming Convention | Purpose |
|---|---|---|
| `AzureAdApplication` | `<applicationRef>` | Registers a client in Azure AD via nais/azurerator (only for `InlineClient` and `None` config types) |
| `Secret` | `<applicationRef>-entraid-<hash>` (inline) or `<securityConfigName>-entraid-<hash>` (ref/secretRef) | Holds Entra ID credentials for the Texas sidecar |
| `ServiceEntry` | `<securityConfigName>-entraid-<hash>` | Allows egress to `login.microsoftonline.com` through Istio |

#### OPA

| Generated Resource | Naming Convention | Purpose |
|---|---|---|
| `ConfigMap` | `<securityConfigName>-opa-<hash>` | Stores fetched OPA bundle content (binary data) for consumption by the OPA sidecar |

### Config Types (Maskinporten & Entra ID)

Both `maskinporten` and `entraid` specs support three mutually exclusive ways to supply credentials (at most one of `client`, `clientRef`, or `secretRef` may be set — enforced by CRD CEL validation):

| Config type | Field | Behaviour |
|---|---|---|
| `InlineClient` | `spec.maskinporten.client` / `spec.entraid.client` | Creates a new `MaskinportenClient` / `AzureAdApplication` managed by Accesserator |
| `ClientRef` | `spec.maskinporten.clientRef` / `spec.entraid.clientRef` | References an existing `MaskinportenClient` / `AzureAdApplication` by name |
| `SecretRef` | `spec.maskinporten.secretRef` / `spec.entraid.secretRef` | Sources credentials directly from keys in existing Kubernetes secrets |
| `None` | (no field set) | Creates a minimal `MaskinportenClient` / `AzureAdApplication` with default naming |

### Status Phases

`SecurityConfig.status.phase` transitions through: `Pending` → `Ready` | `Failed` | `Invalid`.

- **Pending**: Reconciliation has started but is not yet complete.
- **Ready**: All child resources were successfully reconciled.
- **Failed**: One or more child resource reconciliations failed.
- **Invalid**: Validation failed (e.g. OPA is not enabled on the cluster, or a bundle URL is from a disallowed registry).

## Architecture & Code Structure

### Reconciliation Flow

```
SecurityConfigReconciler.Reconcile()
  ├─ Fetch SecurityConfig
  ├─ Apply standard labels
  ├─ resolver.ResolveSecurityConfig()     ← Resolvers (internal/resolver/)
  │   ├─ ResolveTokenXConfig               (fetches Skiperator Application + resolves Jwker inbound rules)
  │   ├─ ResolveMaskinportenConfig         (determines config type; fetches secret data for SecretRef)
  │   ├─ ResolveEntraIdConfig              (determines config type; fetches secret data for SecretRef)
  │   └─ ResolveOpaConfig                 (fetches + verifies OCI bundles; cluster-feature-gated)
  ├─ reconciler.ControllerResources()     ← Builds list of 9 reconcile adapters (internal/reconciler/resources.go)
  │   ├─ Jwker
  │   ├─ TokenX NetworkPolicy (egress)
  │   ├─ MaskinportenClient
  │   ├─ Maskinporten Secret
  │   ├─ Maskinporten ServiceEntry
  │   ├─ AzureAdApplication
  │   ├─ Entra ID Secret
  │   ├─ Entra ID ServiceEntry
  │   └─ OPA ConfigMap
  ├─ doReconcile()                        ← Iterates and executes all adapters
  └─ statusmanager.UpdateSecurityConfigStatus()
```

### Key Packages

| Package | Responsibility |
|---|---|
| `api/v1alpha/` | CRD type definitions with kubebuilder markers; `SecurityConfig`, `SecurityConfigSpec`, all feature specs, status |
| `cmd/main.go` | Entrypoint; scheme registration, manager setup, webhook registration |
| `internal/controller/` | Reconciler loop; resolves, reconciles, updates status |
| `internal/reconciler/` | `ControllerResourceAdapter[T]` — thin wrapper that calls the generic reconciler. `resources.go` defines the 9 adapters |
| `internal/resolver/` | Per-feature resolvers: `tokenx_resolver.go`, `maskinporten_resolver.go`, `entraid_resolver.go`, `opa_resolver.go` |
| `internal/state/` | `Scope` struct — the resolved state bag threaded through all reconciliation (holds config per feature + descendants) |
| `internal/statusmanager/` | Condition building, phase determination, status updates |
| `internal/eventhandler/` | Watches related resources and enqueues affected `SecurityConfig` objects for re-reconciliation: `skiperator_application.go`, `maskinporten_client.go`, `azure_ad_application.go`, `secret.go` |
| `internal/webhook/pods/` | Mutating + validating admission webhook for `Pod`; injects/validates `texas` and `opa` sidecar init containers |
| `internal/webhook/securityconfigs/` | Validating admission webhook for `SecurityConfig`; validates OPA bundle URLs and cosign signatures at admission time |
| `pkg/resourcegenerators/tokenx/` | Desired-state generators for `Jwker` (`jwker/`) and egress `NetworkPolicy` (`egress/`) |
| `pkg/resourcegenerators/maskinporten/` | Desired-state generators for `MaskinportenClient`, `Secret`, `ServiceEntry` |
| `pkg/resourcegenerators/entraid/` | Desired-state generators for `AzureAdApplication`, `Secret`, `ServiceEntry` |
| `pkg/resourcegenerators/opa/` | Desired-state generator for the OPA `ConfigMap` |
| `pkg/reconciliation/` | Generic `ReconcileControllerResource[T]` function + `ControllerResource` interface; handles create/update/delete lifecycle |
| `pkg/validation/` | OPA bundle URL validation (allowed registry prefixes) + cosign/sigstore signature verification via `ValidateBundleSourceSignature` |
| `pkg/utilities/` | Namer helpers (`TokenxNamer`, `MaskinportenNamer`, `EntraIdNamer`, `OpaNamer`), `Ptr()`, `WithShortHashSuffix()`, `DetermineConfigType()`, secret/k8s helpers |
| `pkg/config/` | Env-based config via `envconfig` (prefix `ACCESSERATOR_`); loaded once at startup |
| `pkg/labels/` | Standard labels applied to all child resources |
| `pkg/log/` | Thin logger wrapper with Debug/Info/Warning/Error levels |

### Adapter Pattern for Reconciliation

The reconciler uses a **generic adapter pattern** with Go generics:

```go
ControllerResourceAdapter[T client.Object]
  └─ ReconcilerAdapter[T]
       └─ ResourceReconciler[T]
            ├─ DesiredResource *T      (nil → trigger deletion)
            ├─ ShouldUpdate(current, desired T) bool
            └─ UpdateFields(current, desired T)
```

`ReconcileControllerResource[T]` handles the full lifecycle: **create if not found**, **update if `ShouldUpdate` returns true**, **delete if `DesiredResource` is nil** (and the resource is owned by the SecurityConfig).

### Scope (State Bag)

`state.Scope` is created during resolution and passed through the entire reconciliation:

```go
type Scope struct {
    SecurityConfig         v1alpha.SecurityConfig
    TokenXConfig           TokenXConfig
    MaskinportenConfig     MaskinportenConfig   // Type: InlineClient | ClientRef | SecretRef | None
    EntraIdConfig          EntraIdConfig        // Type: InlineClient | ClientRef | SecretRef | None
    OpaConfig              OpaConfig
    Descendants            []Descendant[client.Object]  // tracks reconciled child resources + status
    InvalidConfig          bool
    ValidationErrorMessage *string
}
```

### OPA Bundle Resolution

OPA bundle resolution (`internal/resolver/opa_resolver.go`) is the most complex resolver:
1. Validates bundle URLs against `ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES`
2. Resolves the manifest digest from the OCI registry (using ORAS)
3. If `spec.opa.bundleUrls[].verification` is set, verifies the cosign/sigstore attestation before pulling layer content
4. Pulls the tar+gzip layer content and stores it in `OpaConfig.BundleBinaryData`

The same signature verification logic runs **both** at admission time (SecurityConfig webhook) and during reconciliation.

### Webhook Logic

**Pod webhook** (`internal/webhook/pods/`):
- Eligibility check: only processes pods in SKIP-managed namespaces (namespace label `skip.kartverket.no/skip-managed: "true"`) that have `accesserator.kartverket.no/` annotations.
- Reads `accesserator.kartverket.no/services` annotation (comma-separated: `texas`, `opa`).
- Looks up the `SecurityConfig` for `applicationRef == pod label application.skiperator.no/app-name`.
- **Mutating**: injects texas/opa as sidecar init containers (Kubernetes 1.33+ `RestartPolicy: Always`).
- **Validating**: verifies injected containers match the expected spec.
- `accesserator.kartverket.no/verify-securityconfig: "true"` annotation triggers a check that exactly one `SecurityConfig` exists for the application, without injecting anything.

**SecurityConfig webhook** (`internal/webhook/securityconfigs/`):
- Validates OPA bundle URLs against allowed registry prefixes.
- Verifies cosign signatures for bundles that specify `spec.opa.bundleUrls[].verification`.

## Coding Conventions

### Go Style

- **Go version**: see `go.mod` (currently 1.26.4)
- **Linter**: golangci-lint with config in `.golangci.yml`. Key enabled linters: `revive`, `gocyclo`, `govet`, `staticcheck`, `errcheck`, `ginkgolinter`, `lll`, `dupl`, `misspell`, `copyloopvar`, `goconst`, `ineffassign`, `nakedret`, `prealloc`, `unconvert`, `unparam`, `unused`.
- **Specific revive rules**: `comment-spacings` (enforces a space after `//`) and `import-shadowing` (disallows shadowing imported package names).
- **Line length**: `lll` is enabled but relaxed for `api/*` and `internal/*` paths (see `.golangci.yml`).
- **Formatting**: `gofmt` + `goimports` enforced via `make fmt`.
- Use `utilities.Ptr(value)` to create pointers from literal values.
- Use structured key-value pairs for log messages: `logger.Info("msg", "key1", value1, "key2", value2)`.
- Name constants at the top of files; avoid magic strings for environment variable names or Kubernetes label/annotation keys.

### Naming Conventions

All child resource names are produced by the namer helpers in `pkg/utilities/helper_functions.go`:

| Resource | Namer method | Pattern |
|---|---|---|
| `Jwker` | `TokenxNamer.JwkerName()` | `<applicationRef>` |
| TokenX `NetworkPolicy` | `TokenxNamer.EgressName(tokenxName)` | `<securityConfigName>-<tokenxName>-egress` |
| `MaskinportenClient` | `MaskinportenNamer.MaskinportenClientName()` | `<applicationRef>` |
| Maskinporten `Secret` (inline) | `MaskinportenNamer.SecretName()` | `<applicationRef>-maskinporten-<hash>` |
| Maskinporten `Secret` (clientRef/secretRef) | `MaskinportenNamer.SecretFromRefName()` | `<securityConfigName>-maskinporten-<hash>` |
| Maskinporten `ServiceEntry` | `MaskinportenNamer.ServiceEntryName()` | `<securityConfigName>-maskinporten-<hash>` |
| `AzureAdApplication` | `EntraIdNamer.AzureAdApplicationName()` | `<applicationRef>` |
| Entra ID `Secret` (inline) | `EntraIdNamer.SecretName()` | `<applicationRef>-entraid-<hash>` |
| Entra ID `Secret` (clientRef/secretRef) | `EntraIdNamer.SecretFromRefName()` | `<securityConfigName>-entraid-<hash>` |
| Entra ID `ServiceEntry` | `EntraIdNamer.ServiceEntryName()` | `<securityConfigName>-entraid-<hash>` |
| OPA `ConfigMap` | `OpaNamer.ConfigMapName()` | `<securityConfigName>-opa-<hash>` |

The `<hash>` suffix is a stable 8-character FNV-32a hex hash of the full unsuffixed name, produced by `WithShortHashSuffix()`.

### CRD / API Changes

When modifying `api/v1alpha/securityconfig_types.go`:
1. Add appropriate `+kubebuilder:validation:*` or `+kubebuilder:validation:XValidation` (CEL) markers.
2. Run `make generate` to regenerate CRD manifests (`config/crd/bases/`) and `zz_generated.deepcopy.go`.
3. Update `api-docs.md` by running `make docs` (requires Docker).
4. Update examples in `examples/` if the change is user-facing.
5. Add Chainsaw e2e tests for new behaviour.

### Kubebuilder Markers

RBAC permissions are declared via `+kubebuilder:rbac` comments on the `Reconcile` method in `securityconfig_controller.go`. When adding new resource types to watch or manage, update both the RBAC markers and `SetupWithManager()`.

## Testing

### Unit/Integration Tests (envtest + Ginkgo)

- Framework: envtest + Ginkgo/Gomega (BDD-style).
- Suite setup files: `internal/controller/suite_test.go`, `internal/resolver/suite_test.go`, etc. — bootstrap envtest with CRDs from `config/crd/bases/` and register all required schemes.
- Run: `make test`
- Tests use a real API server (envtest) but no real cluster.

### End-to-End Tests (Chainsaw)

- Framework: Kyverno Chainsaw for test orchestration.
- Config: `test/chainsaw/config.yaml`.
- Test location: `test/chainsaw/securityconfig/<feature>/<test-name>/`
- Feature directories correspond to the four feature groups: `tokenx/`, `maskinporten/`, `entraid/`, `opa/`, `opa_bundle_verification/`, plus cross-feature directories (`texas_tokenx/`, `texas_maskinporten/`, `texas_entraid/`, `webhook_invocation/`).
- Each test folder typically contains:
  - `chainsaw-test.yaml` — test steps (create resources, assert, delete, etc.)
  - `securityconfig.yaml` — the `SecurityConfig` under test
  - Any supplementary manifests required (Secrets, Skiperator Applications, etc.)
- Run all: `make chainsaw-test-all`
- Run single: `make chainsaw-test-single dir=test/chainsaw/securityconfig/<feature>/<test-name>/`

### Test Naming

Chainsaw test directories use descriptive snake_case names that clearly describe the scenario (e.g., `inline_client`, `client_ref`, `secret_ref`, `texas_sidecar_injected`, `opa_bundle_verified`).

## Technology Stack & Compatibility

### External Operators (via nais/liberator)

Accesserator manages resources owned by two other NAIS operators:
- **[Jwker](https://github.com/nais/jwker)**: watches `Jwker` resources and creates JWK secrets + client registrations in Tokendings.
- **[Digdirator](https://github.com/nais/digdirator)**: watches `MaskinportenClient` resources and registers clients at Digdir's APIs.
- **[Azurerator](https://github.com/nais/azurerator)**: watches `AzureAdApplication` resources and registers clients in Azure AD.

Accesserator only creates/updates/deletes these resources; it does not control the synchronization done by those operators.

### Synchronization States

Accesserator reads the synchronization state set by external operators to determine `SecurityConfig` readiness:

| Resource | Ready state constant |
|---|---|
| `Jwker` | `JwkerSynchronizationStateReady = "RolloutComplete"` |
| `MaskinportenClient` | `MaskinportenClientSynchronizationStateReady = "Synchronized"` |
| `AzureAdApplication` | `AzureAdApplicationSynchronizationStateReady = "Synchronized"` |

### Istio

Maskinporten and Entra ID both create an Istio `ServiceEntry` to allow egress to the respective APIs through the service mesh. Compatible Istio version is determined by `istio.io/client-go` in `go.mod`.

### OCI / Sigstore

OPA bundles are stored as OCI artifacts in container registries. Accesserator uses:
- **[ORAS](https://oras.land/)** (`oras.land/oras-go/v2`) for pulling OCI artifacts.
- **[sigstore-go](https://github.com/sigstore/sigstore-go)** for verifying cosign bundle attestations against Rekor.
- Docker credential store for authenticating against `ghcr.io` and other registries.

### Skiperator Integration

Accesserator reads Skiperator `Application` resources to:
- Resolve `spec.tokenx.accessPolicy.inheritInboundRules` — inheriting inbound access policy rules from the Application's `spec.accessPolicy.inbound.rules`.
- Confirm the referenced application exists before reconciling TokenX.
- Watch all `Application` resources to re-trigger reconciliation when applications change.

### Sidecar Init Containers

Texas and OPA are injected as **sidecar init containers** (using `spec.initContainers[].restartPolicy: Always`), available since Kubernetes 1.33. The common security context for all init containers is defined in `utilities.CommonInitContainer` (`pkg/utilities/constants.go`).

## Configuration

The operator reads its configuration from environment variables at startup, via `pkg/config/config.go` (prefix: `ACCESSERATOR_`):

| Variable | Required | Description |
|---|---|---|
| `ACCESSERATOR_RUNS_IN_PRODUCTION` | Yes | Whether to use production endpoints (e.g. `maskinporten.no` vs `test.maskinporten.no`) |
| `ACCESSERATOR_CLUSTER_NAME` | Yes | Name of the Kubernetes cluster (used in Jwker inbound rules) |
| `ACCESSERATOR_TOKENX_NAMESPACE` | Yes | Namespace where Tokendings runs |
| `ACCESSERATOR_TEXAS_IMAGE_TAG` | Yes | Image tag for the Texas sidecar |
| `ACCESSERATOR_TEXAS_IMAGE_SHA` | Yes | Image digest (sha256) for the Texas sidecar |
| `ACCESSERATOR_ENTRA_TENANT_ID` | Yes | Azure AD tenant ID for Entra ID endpoint construction |
| `ACCESSERATOR_OPA_IMAGE_TAG` | Yes | Image tag for the OPA sidecar |
| `ACCESSERATOR_OPA_IMAGE_SHA` | Yes | Image digest for the OPA sidecar |
| `ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES` | Yes | Comma-separated list of allowed OCI registry URL prefixes for OPA bundles |
| `ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS` | Yes | Comma-separated list of allowed GitHub orgs for OPA bundle signature verification |
| `ACCESSERATOR_OPA_ENABLED` | No | Whether OPA feature is enabled on this cluster (default: `false`) |
| `ACCESSERATOR_TOKENX_NAME` | No | Name of the Tokendings service (default: `tokendings`) |
| `ACCESSERATOR_TEXAS_IMAGE_NAME` | No | Image name for Texas (default: `ghcr.io/nais/texas`) |
| `ACCESSERATOR_OPA_IMAGE_NAME` | No | Image name for OPA (default: `openpolicyagent/opa`) |

## Local Development

### Environment Setup

- **Flox**: Development environment manager (`.flox/`). `flox activate` provisions all tooling.
- **Kind**: Local Kubernetes cluster (cluster name: `accesserator`, context: `kind-accesserator`).
- Components installed in the local cluster: Istio, cert-manager, Skiperator, Tokendings, Jwker, Ztoperator, mock-oauth2-server.
- Environment variables are loaded from `config/manager/base/.env` (created from the cluster setup).
- IDE run configs: `.run/Run accesserator.run.xml`, `.run/Setup accesserator.run.xml` (JetBrains GoLand/IntelliJ).

### Key Make Targets

| Target | What it does |
|---|---|
| `make local` | Full local environment setup (kind cluster + all dependencies) |
| `make run-local` | Run operator from host machine (used by IDE run config) |
| `make deploy` | Build image, load into kind, deploy operator to kind cluster |
| `make ghcr-secret` | Create a Docker pull secret for `ghcr.io` in the cluster (required for OPA bundle fetching when running in-cluster) |
| `make build` | Generate + fmt + vet + compile the manager binary |
| `make generate` | Regenerate CRDs, RBAC, DeepCopy code |
| `make docs` | Regenerate `api-docs.md` from CRD bases (requires Docker) |
| `make test` | Run envtest/Ginkgo unit+integration tests |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Run golangci-lint with auto-fix |
| `make chainsaw-test-all` | Run all Chainsaw e2e tests |
| `make chainsaw-test-single dir=<path>` | Run a single Chainsaw test directory |
| `make clean` | Delete kind cluster |

### Verification Workflow

After making code changes, run the following in order:

1. **If `api/v1alpha/` types changed**: `make generate`
2. **Compile check**: `make build`
3. **Unit tests**: `make test`
4. **Lint**: `make lint` (use `make lint-fix` to auto-fix formatting issues)

## Git Conventions

See [CONTRIBUTING.md#git-conventions](CONTRIBUTING.md#git-conventions).

## Dependency Management

See [CONTRIBUTING.md#managing-dependencies-and-patching-vulnerabilities](CONTRIBUTING.md#managing-dependencies-and-patching-vulnerabilities).

## CI/CD

- **Build & Deploy**: `.github/workflows/` — builds container image, pushes to `ghcr.io`.
- **Tests**: Ginkgo unit tests and Chainsaw e2e tests run in CI.
- **Lint**: golangci-lint via GitHub Actions.
- **Dependency updates**: Dependabot for `gomod` and `github-actions`.

## Important Constraints

1. **Never manually edit** `config/crd/bases/` or `zz_generated.deepcopy.go` — these are generated by `make generate`.
2. **OPA is cluster-feature-gated** — `spec.opa` can only be set when `ACCESSERATOR_OPA_ENABLED=true` on the cluster. Both the controller and the SecurityConfig webhook enforce this.
3. **At most one of `client`, `clientRef`, or `secretRef`** may be set for Maskinporten/Entra ID — enforced by CRD CEL validation at admission time.
4. **Exactly one `SecurityConfig` per Skiperator Application** — pods annotated with `accesserator.kartverket.no/verify-securityconfig: "true"` will be denied if this is not satisfied.
5. **The operator only manages resources it owns** — it will refuse to update or delete resources not owned by the SecurityConfig (i.e., those not created by Accesserator).
6. **OPA bundle signatures are verified at two points**: at admission (SecurityConfig webhook) and during reconciliation. Both use the same `ValidateBundleSourceSignature` function from `pkg/validation/`.
7. **Sidecar init containers require Kubernetes 1.33+** — the `RestartPolicy: Always` field on init containers used for texas/opa injection is only available from that version onward.
8. **Watching Skiperator Applications, Maskinporten/Azure AD client resources, and Secrets** will trigger re-reconciliation of all `SecurityConfig` objects that reference the changed resource.

## Keeping Documentation Up-to-Date

- Update this document when making significant changes to architecture, code structure, feature groups, or conventions.
- Keep references to package names, resource types, naming patterns, and version numbers accurate as the code evolves.
- Update `api-docs.md` with `make docs` after any CRD type changes.

