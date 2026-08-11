# Contributing guides
## Getting started

To run Accesserator locally, you need to have go >= 1.25.7 installed on your machine. 
You also need some other tools to set up and interact with the local Kubernetes cluster. All these 
dependencies are bundled together with a [Flox environment](https://flox.dev/), which is a tool that provides a consistent development environment across different machines. 
You can install Flox by following the instructions on their [website](https://flox.dev/docs/install-flox/install).

To activate the Flox environment for this project, run the following command in the terminal:
```bash
flox activate
```
This will set up a local Kubernetes cluster (with [kind](https://kind.sigs.k8s.io/)) as well as give access to useful tools like `kubectl`, `kubectx`, `k9s`, `cloud-provider-kind` and `kubefwd`. 
The local Kubernetes cluster will have the following components installed and configured:
- Istio as service mesh
- Cert-manager to manage TLS certificates for webhook communication
- Skiperator to manage the lifecycle of the workloads deployed in the cluster
- Tokendings to act as an internal token exchange server
- Jwker to manage OAuth2 client registrations for applications that use tokendings
- Ztoperator to handle OAuth2 authorization code flow and JWT verification and authorization for workloads in the cluster
- Mock-OAuth2-Server to mock an external identity provider for testing purposes.

## Run accesserator locally

### Run on your host machine

In order for accesserator to work properly with OPA, you need to login with docker to be able to fetch OPA bundles from ghcr.io. 
This can be done by running the following command in the terminal:
```bash
docker login ghcr.io
```

To run accesserator with the local cluster, you can press `Run` on the run configuration called `Run accesserator` (if you have the project open in a JetBrains IDE), 
or run the following command in the terminal (where you previously activated the [Flox environment](#getting-started)):
```bash
make run-local
```

### Run in the local cluster

In order for accesserator to work properly with OPA, you need to login with docker to be able to fetch OPA bundles from ghcr.io.
This can be done by running the following command in the terminal:
```bash
make ghcr-secret
```

To run accesserator in the local cluster, you need to build and deploy a local image of accesserator to the cluster. You can do this by running the following command in the terminal (where you previously activated the [Flox environment](#getting-started)):
```bash
make deploy
```

You can then verify that everything is working correctly by applying the example `SecurityConfig` + Skiperator `Application` from the [examples](examples) folder.

```bash
kubectl apply -f examples/example.yaml
```

## Running tests

> [!IMPORTANT]  
> Some of the Chainsaw tests fetches OCI artifacts from GHCR. In order for this to succeed when running accesserator on your host machine, you need to login with docker to be able to fetch OPA bundles from ghcr.io.
> This can be done with `docker login ghcr.io`. In order for this to succeed when running accesserator in the local cluster, you need to create a secret with `make ghcr-secret`.

> [!IMPORTANT]  
> The chainsaw test `opa_bundle_verification_private_package` fetches a private GitHub package from GHCR. In order for this test to work you need access to pull this package from GHCR. 
> For it to work in GitHub Actions, the private GitHub package needs to specify kartverket/accesserator as repository which can access the package in GitHub Actions. This is done under "Manage Actions access" in "Package settings" for the private package in GitHub.
> The default GitHub token used in github actions (`${{ secrets.GITHUB_TOKEN }}`) also needs to have access to read packages. This can be done under "Actions" -> "General" -> "Workflow permissions" in the repository settings for the private package in GitHub.

We use [envtest](https://book.kubebuilder.io/reference/envtest) and [Ginko](https://onsi.github.io/ginkgo/) for our unit and integration tests, as well as [chainsaw](https://kyverno.github.io/chainsaw/0.2.3/) for end-to-end testing.

To run the Ginko tests locally, run the following command in the terminal:
```bash
make test
```

Run all end-to-end tests in parallel with
```bash
make chainsaw-test-all
```
or run a single test with
```bash
make chainsaw-test-single dir=<TEST FOLDER>
```

## Git conventions

### Commit strategy

- Separate features, bug fixes, and refactorings into different commits.
- Each commit should represent one logical change that compiles and passes tests on its own.
- Do not mix refactoring or unrelated changes with feature changes in the same commit.
- Keep changes and their corresponding tests in the same commit.

### Commit messages

- Limit the subject line to 50 characters.
- Capitalize the subject line.
- Use the imperative mood in the subject line.
- Do not end the subject line with a period.
- Separate the subject from the body with a blank line.
- Wrap the body at 72 characters.
- Use the body to explain what and why, not how.

Example:
```
Add secretRef support for Entra ID config

Previously, Entra ID credentials could only be configured via an
inline AzureAdApplication or by referencing an existing one. This
adds a third path: sourcing credentials directly from existing
Kubernetes secrets, mirroring the existing Maskinporten secretRef
behaviour.
```

## Managing dependencies and patching vulnerabilities

### Managing Kubernetes, Istio and CertManager versions
These versions are defined in the top of the `Makefile`, and are used when running locally and when testing in CI/CD.
Versions should match the versions running in your production Kubernetes cluster.

### Managing helper tools
Versions for helper tools like Kustomize, Chainsaw, Kubectl etc. are defined in the top of the `Makefile`. Versions
should be updated either regularly, or as is required when updating other dependencies (such as upgrading Chainsaw when
updating Go version).

### Updating Go version
#### Updating Go version
1. Find a desired stable Go version [on the go.dev website](https://go.dev/dl/#stable)
2. Investigate changes between current and desired version, for instance through [the go release history page](https://go.dev/doc/devel/release),
   or by googling "Go <old_version> to <new_version> migration guide"
3. Ensure there exists a compatible Chainsaw version for the chosen Go version, for instance by checking [the latest Chainsaw releases](https://github.com/kyverno/chainsaw/releases).
   The version of Chainsaw must support the same major and minor version as your chosen Go version. Patch versions may
   typically be updated without updating Chainsaw. If necessary, update `CHAINSAW_VERSION` in the `Makefile`
4. Update Go version in `go.mod`
5. Perform dependency updates for direct dependencies with `go get -u ./...`
6. Revert any changes made to `github.com/nais/liberator`. Liberator must be updated separately.
7. Run `go mod tidy`

#### Updating Golang base image version
1. Find a Golang docker image [on Docker Hub](https://hub.docker.com/_/golang/tags) corresponding to the Go version you
   updated to previously. As of August 2026, this is called "1.26.5-alpine3.24".
2. Copy the "index digest" (top left), and paste into all relevant dockerfiles:
   3. hack/mock_controller/Dockerfile
   4. Dockerfile
   5. Dockerfile.goreleaser

#### Verify linting and tests
1. Run `make lint`. This ensures both that the linting is correct, and that the version of `golangci-lint` supports the
   new Go version. If the version of `golangci-lint` does not support the new version of Go, update
   `GOLANGCI_LINT_VERSION` in the `Makefile`. If there are linting errors after updating the linter version, solve these
   either by running `make lint-fix` or resolving manually
2. Follow guides above to run tests (with `make test`) and e2e-tests (with `make chainsaw-test-all`), and ensure all
   tests pass
