#!/bin/bash

kind create cluster --image kindest/node:v1.35.0

make install-istio-gateways skiperator tokendings jwker

make docker-build IMG=accesserator:v0

kind load docker-image accesserator:v0 --name kind

make deploy IMG=accesserator:v0

kubectl apply -f examples/example.yaml