# Mock Controller

Mock controller is a simple Kubernetes controller 
that serves as a mock for testing and development purposes. 
It watches for `MaskinportenClients` and creates corresponding 
Kubernetes Secrets holding the client integration secrets towards maskinporten.

## Usage

Before running the mock controller you have to set up a .env file with the following environment variables:

```shell
MASKINPORTEN_CLIENT_ID=1fc6afa7-eafb-4d3e-9b6f-a7dc958d46d3 # An actual client registered in maskinporten (test)
MASKINPORTEN_CLIENT_JWK={"use":"sig","kty":"RSA","kid":"a51fd3e9-7e10-4acf-bd60-0e72e2e60f2d","alg":"RS256","n":"...","e":"AQAB"}
```

The controller can either run locally through the run configuration `Run mock-controller` in your IDE, or it can be deployed and 
run as a deployment in a local cluster.

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
make -C "$REPO_ROOT" deploy-mock-controller
```