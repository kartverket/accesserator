# OPA Policies

This directory contains OPA (Open Policy Agent) policies for testing and development purposes.

## Building the Bundle

To build an OPA bundle from this directory:

```bash
opa build -b .opa -o bundle.tar.gz
```

## Testing Policies

To test the policies:

```bash
opa test .opa -v --fail-on-empty
```

## Running OPA locally

### Run OPA
```bash
# Run OPA with the policy
opa run --server --addr 0.0.0.0:8181 --log-level debug --log-format text --watch --set decision_logs.console=true .opa/authz
```

### Test a request
curl -X POST http://localhost:8181/v1/data/authz/allow \
  -H "Content-Type: application/json" \
  -d '{"input": {"password": "accesserator"}}'
```