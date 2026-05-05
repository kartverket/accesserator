# Image URL to use all building/pushing image targets
IMG ?= accesserator:latest
MOCK_CONTROLLER_IMG ?= mock-controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

##@ Variables

KUBERNETES_VERSION			= 1.35.0
KIND_IMAGE					= kindest/node:v$(KUBERNETES_VERSION)
KIND_CLUSTER_NAME          ?= accesserator
KUBECONTEXT                ?= kind-$(KIND_CLUSTER_NAME)
ISTIO_VERSION 				= 1.28.0
CERT_MANAGER_VERSION		= 1.19.2
LOCAL_WEBHOOK_CERTS_DIR	   ?= webhook-certs

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: run-local
run-local: ensurelocal ensureaccesseratornotdeployed generate install webhooks sourceenv ## Run Accesserator from your host.
	go run ./cmd/main.go -webhook-cert-path=./webhook-certs

.PHONY: isrunning
isrunning: ## Check if accesserator is running on your host machine (i.e. from IDE or with 'make run-local')
	@echo "Checking if accesserator is running..."
	@lsof -i :8081 > /dev/null || (echo "❌ accesserator is not running. Please start it first either in your IDE or with 'make run-local'." && exit 1)
	@echo "✅ accesserator is running."

.PHONY: isnotrunning
isnotrunning: ## Check if accesserator is NOT running on your host machine (i.e. from IDE or with 'make run-local')
	@echo "Checking if accesserator is not running..."
	@lsof -i :8081 > /dev/null || (echo "✅ accesserator is not running on your host. Ready to deploy." && exit 0 || echo "❌ accesserator is running on your host. Please stop it first." && exit 1)
	@echo "✅ accesserator is not running."

.PHONY: ismockcontrollerrunning
ismockcontrollerrunning: ## Check if mock-controller is running on your host machine (i.e. from IDE or with 'go run')
	@echo "Checking if mock-controller is running..."
	@lsof -i :8083 > /dev/null || (echo "❌ mock-controller is not running. Please start it first either in your IDE or with 'go run ./hack/mock_controller/main.go'." && exit 1)
	@echo "✅ mock-controller is running."

.PHONY: ismockcontrollernotrunning
ismockcontrollernotrunning: ## Check if mock-controller is NOT running on your host machine
	@echo "Checking if mock-controller is not running..."
	@! lsof -i :8083 > /dev/null 2>&1 || (echo "❌ mock-controller is running on your host. Please stop it first." && exit 1)
	@echo "✅ mock-controller is not running on your host. Ready to deploy."

.PHONY: sourceenv
sourceenv: ## Source environment variables from config/manager/base/.env file
	@set -a; [ -f config/manager/base/.env ] && . config/manager/base/.env; set +a

.PHONY: local
local: cluster accesserator-namespace cert-manager istio-gateways skiperator mock-oauth2 tokendings jwker deploy-mock-controller generate install ## Set up entire local development environment with external dependencies

.PHONY: clean
clean: kind ## Clean up local environment by deleting kind cluster
	"$(KIND)" delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" rbac:roleName=accesserator crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases output:webhook:artifacts:config=config/webhook/bases

.PHONY: docs
docs: ## Generate API documentation from CRD bases using crdoc
	docker run --platform linux/amd64 \
	  -u $$(id -u):$$(id -g) --rm \
	  -v $$PWD:/workdir \
	  ghcr.io/fybrik/crdoc:latest \
	  --resources /workdir/config/crd/bases \
	  --output /workdir/api-docs.md

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: generate fmt vet setup-envtest ## Run go tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./...) -coverprofile cover.out

.PHONY: chainsaw-test-all
chainsaw-test-all: chainsaw install ensurelocal ensurerunningordeployed ensuremockoauth2isreachable ## Run all chainsaw tests in parallel
	@/bin/bash ./scripts/chainsaw-test-all.sh

.PHONY: chainsaw-test-single
chainsaw-test-single: chainsaw-ensure-dir-set chainsaw install ensurelocal ensurerunningordeployed ensuremockoauth2isreachable ## Run a specific chainsaw test. Example usage: make chainsaw-test-single dir=<CHAINSAW_TEST_DIR>
	@/bin/bash ./scripts/chainsaw-test-single.sh -d $(dir)

.PHONY: chainsaw-ensure-dir-set
chainsaw-ensure-dir-set: ## Ensure that the 'dir' variable is set when running 'make chainsaw-test-single'
	$(if $(strip $(dir)),,$(error dir is not set. Usage: make chainsaw-test-single dir=<CHAINSAW_TEST_DIR>))

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run --config .golangci.yml

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix --config .golangci.yml

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/accesserator cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-build-mock-controller
docker-build-mock-controller: ## Build docker image for mock-controller.
	$(CONTAINER_TOOL) build -t ${MOCK_CONTROLLER_IMG} -f hack/mock_controller/Dockerfile .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: deploy
deploy: ensurelocal isnotrunning accesserator-namespace generate install kustomize docker-build ## Deploy accesserator and all the required resources for accesserator to run properly to the kind cluster
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KIND)" load docker-image ${IMG} --name $(KIND_CLUSTER_NAME)
	"$(KUSTOMIZE)" build config/webhook | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -
	"$(KUSTOMIZE)" build config/manager | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=30s -n accesserator-system -l app=accesserator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying accesserator." && exit 1)
	@echo -e "✅  accesserator installed in namespace 'accesserator-system'!"

.PHONY: deploy-mock-controller
deploy-mock-controller: mock-controller-namespace docker-build-mock-controller ## Deploy mock-controller and all the required resources for accesserator to run properly to the kind cluster
	"$(KIND)" load docker-image ${MOCK_CONTROLLER_IMG} --name $(KIND_CLUSTER_NAME)
	@ENV_FILE=hack/mock_controller/.env; \
	if [ ! -f "$$ENV_FILE" ]; then \
		echo "⚠️  hack/mock_controller/.env not found. Falling back to hack/mock_controller/.env.example (dummy values)."; \
		ENV_FILE=hack/mock_controller/.env.example; \
	fi; \
	if [ ! -f "$$ENV_FILE" ]; then \
		echo "❌ Neither hack/mock_controller/.env nor hack/mock_controller/.env.example found."; \
		exit 1; \
	fi; \
	if "$(KUBECTL)" get secret mock-controller-env -n mock-controller-system --context $(KUBECONTEXT) >/dev/null 2>&1; then \
		echo "⏳ Updating existing mock-controller-env secret..."; \
		"$(KUBECTL)" create secret generic mock-controller-env --from-env-file="$$ENV_FILE" -n mock-controller-system --context $(KUBECONTEXT) --dry-run=client -o yaml | \
		"$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; \
	else \
		echo "⏳ Creating mock-controller-env secret..."; \
		"$(KUBECTL)" create secret generic mock-controller-env --from-env-file="$$ENV_FILE" -n mock-controller-system --context $(KUBECONTEXT); \
	fi
	@echo -e "🤞  Installing mock-controller..."
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/install-mock-controller.sh
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=30s -n mock-controller-system -l app=mock-controller --context $(KUBECONTEXT) || (echo -e "❌  Error deploying mock-controller." && exit 1)
	@echo -e "✅  mock-controller installed in namespace 'mock-controller-system'!"

.PHONY: undeploy
undeploy: kustomize ## Undeploy accesserator and all the resources deployed by accesserator to the kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No Webhook configurations to delete; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/manager 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No manager resources to delete; skipping."; fi

.PHONY: undeploy-mock-controller
undeploy-mock-controller: kubectl ## Undeploy mock-controller and all the resources deployed by mock-controller to the kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@if [ -n "$$($(KUBECTL) get deployments -n mock-controller-system --context $(KUBECONTEXT) -o name 2>/dev/null)" ]; then \
		"$(KUBECTL)" delete deployment mock-controller -n mock-controller-system --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found); \
	else \
		echo "No deployments found in mock-controller-system; skipping."; \
	fi

.PHONY: webhooks
webhooks: kustomize ## Extract webhook certificate details
	@/bin/bash ./scripts/get-webhook-certs.sh

.PHONY: install
install: kustomize generate ## Install CRDs, Webhook configurations and ClusterRoles into the local kind cluster.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No CRDs to install; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No ClusterRoles to install; skipping."; fi
	@if $(MAKE) ensureaccesseratornotdeployed >/dev/null 2>&1; then \
		echo "Accesserator is not deployed; installing local webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook-local 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No Webhook configurations to install; skipping."; fi; \
	else \
		echo "Accesserator is deployed; installing cluster webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No Webhook configurations to install; skipping."; fi; \
	fi

.PHONY: uninstall
uninstall: generate kustomize kubectl ## Uninstall CRDs, Webhook configurations and ClusterRoles from the local kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No ClusterRoles to delete; skipping."; fi
	@if $(MAKE) ensureaccesseratornotdeployed >/dev/null 2>&1; then \
		echo "Accesserator is not deployed; uninstalling local webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook-local 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No Webhook configurations to delete; skipping."; fi; \
	else \
		echo "Accesserator is deployed; uninstalling cluster webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No Webhook configurations to delete; skipping."; fi; \
	fi

##@ Cluster

.PHONY: cluster
cluster: kind ## Create Kind cluster with kube context kind-accesserator
	@echo Create kind cluster... >&2
	"$(KIND)" create cluster --image $(KIND_IMAGE) --name ${KIND_CLUSTER_NAME}

##@ Namespace

.PHONY: accesserator-namespace
accesserator-namespace: ## Create accesserator-system namespace in the cluster
	@$(MAKE) create-namespace namespace=accesserator-system

.PHONY: jwker-namespace
jwker-namespace: kubectl ## Create jwker-system namespace in the cluster
	@$(MAKE) create-namespace namespace=jwker-system

.PHONY: tokendings-namespace
tokendings-namespace: kubectl ## Create tokenx-api namespace in the cluster
	@$(MAKE) create-namespace namespace=tokenx-api

.PHONY: mock-controller-namespace
mock-controller-namespace: kubectl ## Create mock-controller-system namespace in the cluster
	@$(MAKE) create-namespace namespace=mock-controller-system

##@ Operators

.PHONY: install-jwker-crds
install-jwker-crds: ## Installing Jwker CRDs
	@echo -e "🤞  Installing jwker cluster resources..."
	@out="$$( "$(KUSTOMIZE)" build config/jwker/cluster-resources 2>/dev/null || true )"; \
    if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No jwker cluster resources to install; aborting." && exit 1; fi

.PHONY: jwker
jwker: kustomize install-jwker-crds jwker-namespace ## Installing Jwker on k8s cluster
	@echo -e "🤞  Installing jwker controller..."
	@out="$$( "$(KUSTOMIZE)" build config/jwker/controller 2>/dev/null || true )"; \
    if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f - -n jwker-system; else echo "No jwker controller to install; aborting." && exit 1; fi
	"$(KUBECTL)" wait pod --for=create --timeout=60s -n jwker-system -l app=jwker --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Jwker." && exit 1; }
	"$(KUBECTL)" wait pod --for=condition=Ready --timeout=60s -n jwker-system -l app=jwker --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Jwker." && exit 1; }
	@echo -e "✅  Jwker installed in namespace 'jwker-system'!"

.PHONY: skiperator
skiperator: ## Install Skiperator on k8s cluster
	@echo -e "🤞  Installing Skiperator..."
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/install-skiperator.sh
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=30s -n skiperator-system -l app=skiperator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying Skiperator." && exit 1)
	@echo -e "✅  Skiperator installed in namespace 'skiperator-system'!"

.PHONY: install-istio
install-istio: ## Install istio
	@echo "⬇️ Downloading Istio..."
	@curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) TARGET_ARCH=$(ARCH) sh -
	@echo "⛵️  Installing Istio on Kubernetes cluster..."
	@./istio-$(ISTIO_VERSION)/bin/istioctl install --context $(KUBECONTEXT) -y --set meshConfig.accessLogFile=/dev/stdout --set profile=minimal
	@rm -rf istio-$(ISTIO_VERSION)
	@echo "✅  Istio installation complete."

.PHONY: istio-gateways
istio-gateways: istiohelm install-istio ## Install istio gateways
	@echo "⛵️ Creating istio-gateways namespace..."
	@kubectl create namespace istio-gateways --context $(KUBECONTEXT) &> /dev/null || true
	@echo "⬇️  Installing istio-gateways"
	"$(HELM)" install istio-ingressgateway istio/gateway --version v$(ISTIO_VERSION) -n istio-gateways --kube-context $(KUBECONTEXT) --set labels.app=istio-ingress-external --set labels.istio=ingressgateway
	@echo "✅  Istio gateways installed."

.PHONY: cert-manager
cert-manager: kustomize kubectl ## Install cert-manager to the cluster
	@echo -e "🤞  Installing cert-manager..."
	"$(KUBECTL)" apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "🕑  Waiting for cert-manager to be ready..."
	"$(KUBECTL)" -n cert-manager wait deploy --all --for=condition=Available --timeout=60s
	@echo -e "✅  Cert-manager installed!"
	@out="$$( "$(KUSTOMIZE)" build config/cert-manager 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No cert manager resources to install; skipping."; fi

##@ Helper services

.PHONY: tokendings
tokendings: kubectl kustomize tokendings-namespace tokendings-database ## Deploying tokendings oauth authorization server
	@echo -e "🤞  Setting up Tokendings..."
	@out="$$( "$(KUSTOMIZE)" build config/tokendings 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f - -n tokenx-api; else echo "No tokendings resources to install; aborting." && exit 1; fi
	@$(MAKE) wait-for-skiperator-pod app=tokendings namespace=tokenx-api
	@echo -e "✅  Tokendings installed in namespace 'tokenx-api'!"

.PHONY: tokendings-database
tokendings-database: kubectl
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash scripts/setup-tokendings-database.sh

.PHONY: mock-oauth2
mock-oauth2: kubectl ## Deployinh Mock-OAuth service in auth namespace
	@echo -e "🤞  Deploying 'mock-oauth2'..."
	@KUBECONTEXT=$(KUBECONTEXT) MOCK_OAUTH2_CONFIG=scripts/mock-oauth2-server-config.json /bin/bash ./scripts/install-mock-oauth2.sh
	@$(MAKE) wait-for-skiperator-pod app=mock-oauth2 namespace=auth
	@echo -e "✅  'mock-oauth2' is ready and running"

##@ Helpers

.PHONY: mock-oauth2-ingress
mock-oauth2-ingress: kubefwd ## Ensure mock-oauth2 is reachable via kubefwd, restarting it if necessary
	@KUBEFWD=$$(command -v kubefwd 2>/dev/null || echo "$(KUBEFWD)"); \
	if [ ! -x "$$KUBEFWD" ]; then \
		echo -e "❌  kubefwd not found. Install it or run 'make kubefwd'."; \
		exit 1; \
	fi; \
	echo -e "🔍  Checking if mock-oauth2 is running in the cluster..."; \
	"$(KUBECTL)" wait pod --for=condition=Ready --timeout=10s -n auth -l app=mock-oauth2 --context $(KUBECONTEXT) 2>/dev/null || { \
		echo -e "❌  mock-oauth2 is not ready. Deploy it first with 'make mock-oauth2'."; \
		exit 1; \
	}; \
	echo -e "✅  mock-oauth2 is ready"; \
	STATUS=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://mock-oauth2.auth:8080/accesserator/.well-known/openid-configuration 2>/dev/null || echo "000"); \
	if [ "$$STATUS" = "200" ]; then \
		echo -e "✅  mock-oauth2 is reachable on http://mock-oauth2.auth:8080"; \
		exit 0; \
	fi; \
	echo -e "⚠️   mock-oauth2 is not reachable (HTTP $$STATUS). Starting kubefwd..."; \
	if pgrep -x "kubefwd" >/dev/null 2>&1; then \
		echo -e "⏳  Stopping existing kubefwd..."; \
		sudo -E pkill -x "kubefwd" || true; \
		sleep 1; \
	fi; \
	LOG=/tmp/kubefwd.log; \
	sudo -E "$$KUBEFWD" svc -n auth --context $(KUBECONTEXT) 2>&1 | tee "$$LOG" > /dev/null & \
	echo -e "⏳  Waiting for kubefwd to establish connections..."; \
	for i in $$(seq 1 15); do \
		sleep 2; \
		STATUS=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 http://mock-oauth2.auth:8080/accesserator/.well-known/openid-configuration 2>/dev/null || echo "000"); \
		if [ "$$STATUS" = "200" ]; then \
			echo -e "✅  mock-oauth2 is now reachable on http://mock-oauth2.auth:8080"; \
			exit 0; \
		fi; \
	done; \
	echo -e "❌  mock-oauth2 is still not reachable. Check $$LOG for details."; \
	exit 1

.PHONY: mock-token
mock-token: ## Retrieves a JWT issued by mock-oauth2
	@JQ_OUTPUT=$$($(MAKE) jq 2>&1); \
	if [ $$? -ne 0 ]; then echo "$$JQ_OUTPUT"; exit 1; fi
	@ENSURE_OUTPUT=$$($(MAKE) ensuremockoauth2isreachable 2>&1); \
	if [ $$? -ne 0 ]; then echo "$$ENSURE_OUTPUT"; exit 1; fi
	@token=$$(curl -s -X POST "http://mock-oauth2.auth:8080/accesserator/token" \
		-d "grant_type=authorization_code" \
		-d "code=code" \
		-d "client_id=something" | "$(JQ)" -r '.access_token // empty'); \
	if [ -z "$$token" ]; then \
		echo -e "❌  No access_token found in response" >&2; \
		exit 1; \
	fi; \
	echo "$$token"

.PHONY: ensurelocal
ensurelocal: kind kubectl ## Ensure local environment is set up with necessary tools and kind cluster is running
	@/bin/bash ./scripts/ensure-local-setup.sh

.PHONY: ensureaccesseratornotdeployed
ensureaccesseratornotdeployed: kubectl ## Ensure accesserator is NOT deployed in the kind cluster
	@if "$(KUBECTL)" -n accesserator-system get deployment accesserator >/dev/null 2>&1; then \
		echo "❌ Accesserator IS deployed to the cluster"; \
		exit 1; \
	else \
		echo "✅ Accesserator IS NOT deployed to the cluster"; \
	fi

.PHONY: ensureaccesseratordeployed
ensureaccesseratordeployed: kubectl ensurelocal isnotrunning ## Ensure accesserator is deployed in the kind cluster
	@/bin/bash ./scripts/ensure-accesserator-deployed.sh || (echo "❌ Accesserator resources are not deployed correctly to the cluster. To fix it, run 'make deploy'." && exit 1)

.PHONY: ensurerunningordeployed
ensurerunningordeployed: ## Ensure accesserator is running on host OR deployed in cluster, but not both
	@$(MAKE) isrunning >/dev/null 2>&1 && running=1 || running=0; \
	$(MAKE) ensureaccesseratordeployed >/dev/null 2>&1 && deployed=1 || deployed=0; \
	if [ "$$running" = "1" ] && [ "$$deployed" = "1" ]; then \
		echo "❌ Accesserator is both running on the host AND deployed in the cluster. Stop one before continuing."; \
		exit 1; \
	fi; \
	if [ "$$running" = "0" ] && [ "$$deployed" = "0" ]; then \
		echo "❌ Accesserator is neither running on the host nor deployed in the cluster."; \
		echo "   Start it in your IDE / with 'make run-local', or deploy it with 'make deploy'."; \
		exit 1; \
	fi; \
	if [ "$$running" = "1" ]; then echo "✅ Accesserator is running on the host."; fi; \
	if [ "$$deployed" = "1" ]; then echo "✅ Accesserator is deployed an running in the local cluster."; fi

.PHONY: ensuremockcontrollernotdeployed
ensuremockcontrollernotdeployed: kubectl ## Ensure mock-controller is NOT deployed in the kind cluster
	@if "$(KUBECTL)" -n mock-controller-system get deployment mock-controller >/dev/null 2>&1; then \
		echo "❌ mock-controller IS deployed to the cluster"; \
		exit 1; \
	else \
		echo "✅ mock-controller IS NOT deployed to the cluster"; \
	fi

.PHONY: ensuremockcontrollerdeployed
ensuremockcontrollerdeployed: kubectl ismockcontrollernotrunning ## Ensure mock-controller is deployed and ready in the kind cluster
	@echo "🔎 Checking mock-controller deployment in mock-controller-system..."; \
	"$(KUBECTL)" get deployment -n mock-controller-system mock-controller >/dev/null 2>&1 || { \
		echo "❌ mock-controller deployment not found. Deploy it first with 'make deploy-mock-controller'."; \
		exit 1; \
	}; \
	READY=$$($(KUBECTL) get deployment -n mock-controller-system mock-controller -o jsonpath='{.status.readyReplicas}' 2>/dev/null); \
	DESIRED=$$($(KUBECTL) get deployment -n mock-controller-system mock-controller -o jsonpath='{.spec.replicas}' 2>/dev/null); \
	if [ "$${READY:-0}" != "$$DESIRED" ]; then \
		echo "❌ mock-controller deployment not ready (ready=$${READY:-0}, desired=$$DESIRED). Run 'make deploy-mock-controller' to fix it."; \
		exit 1; \
	fi; \
	echo "✅ mock-controller is deployed and ready (replicas=$$DESIRED)."

.PHONY: webhook-test-manifests
webhook-test-manifests: kustomize ## Build webhook manifests for envtest into webhook-tests/
	@mkdir -p webhook-tests
	@"$(KUSTOMIZE)" build config/webhook > webhook-tests/webhook-manifests.yaml

##@ Dependencies

.PHONY: istiohelm
istiohelm: helm ## Fetch helm charts for Istio
	# Ensure istio helm repo exists
	"$(HELM)" repo list | grep -q '^istio\s' || (echo "Adding istio helm repo..." && helm repo add istio https://istio-release.storage.googleapis.com/charts)
	# Make sure the requested ISTIO_VERSION is available; update index if not
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "Updating Helm repos to fetch Istio charts..." && helm repo update)
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "❌ Istio Helm chart version $(ISTIO_VERSION) not found in repo index." && echo "   Tip: check available versions with: helm search repo istio/gateway --versions" && exit 1)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CHAINSAW ?= $(LOCALBIN)/chainsaw
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
HELM ?= $(LOCALBIN)/helm
KUBEFWD ?= $(LOCALBIN)/kubefwd
JQ ?= $(LOCALBIN)/jq

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CHAINSAW_VERSION ?= v0.2.14
CONTROLLER_TOOLS_VERSION ?= v0.19.0
KUBECTL_VERSION ?= v1.34.2
KIND_VERSION ?= v0.30.0
GOLANGCI_LINT_VERSION ?= v2.10.1
HELM_VERSION ?= v4.0.0
KUBEFWD_VERSION ?= 1.25.12
JQ_VERSION ?= 1.8.1

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: jq
jq: $(JQ) ## Download jq locally if necessary.
$(JQ): $(LOCALBIN)
	@set -e; \
	if [ -x "$(JQ)" ]; then \
		echo "✅ jq already exists at $(JQ)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	case "$$os" in \
		linux) binary="jq-linux-$${arch}" ;; \
		darwin) binary="jq-macos-$${arch}" ;; \
		*) echo "❌ Unsupported OS: $$os" >&2; exit 1 ;; \
	esac; \
	url="https://github.com/jqlang/jq/releases/download/jq-$(JQ_VERSION)/$${binary}"; \
	echo "Downloading jq $(JQ_VERSION) from $$url"; \
	curl -L -o "$(JQ)" "$$url"; \
	chmod +x "$(JQ)"; \
	echo "✅ jq installed at $(JQ)"
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: kubefwd
kubefwd: $(KUBEFWD) ## Download kubefwd locally if necessary.
$(KUBEFWD): $(LOCALBIN)
	@set -e; \
	if [ -x "$(KUBEFWD)" ]; then \
		echo "✅ kubefwd already exists at $(KUBEFWD)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=x86_64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	case "$$os" in \
		linux) os_name=Linux ;; \
		darwin) os_name=Darwin ;; \
		*) echo "❌ Unsupported OS: $$os" >&2; exit 1 ;; \
	esac; \
	url="https://github.com/txn2/kubefwd/releases/download/v$(KUBEFWD_VERSION)/kubefwd_$${os_name}_$${arch}.tar.gz"; \
	echo "Downloading kubefwd $(KUBEFWD_VERSION) from $$url"; \
	curl -L -o kubefwd.tar.gz "$$url"; \
	tar -xzf kubefwd.tar.gz -C "$(LOCALBIN)" kubefwd; \
	chmod +x "$(KUBEFWD)"; \
	rm kubefwd.tar.gz; \
	echo "✅ kubefwd installed at $(KUBEFWD)"

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(LOCALBIN)
	@set -e; \
	if [ -x "$(HELM)" ]; then \
		echo "✅ helm already exists at $(HELM)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		armv7l) arch=arm ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	url="https://get.helm.sh/helm-$(HELM_VERSION)-$${os}-$${arch}.tar.gz"; \
	echo "Downloading helm $(HELM_VERSION) from $$url"; \
	curl -L -o helm.tar.gz "$$url"; \
	tar -xzf helm.tar.gz -C "$(LOCALBIN)" --strip-components=1 --no-same-owner "$${os}-$${arch}/helm"; \
	chmod +x "$(HELM)"; \
	rm helm.tar.gz; \
	echo "✅ helm installed at $(HELM)"

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	$(call go-install-tool,$(CHAINSAW),github.com/kyverno/chainsaw,$(CHAINSAW_VERSION))

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary.
$(KUBECTL): $(LOCALBIN)
	@set -e; \
	if [ -x "$(KUBECTL)" ]; then \
		echo "✅ kubectl already exists at $(KUBECTL)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		armv7l) arch=arm ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	url="https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$${os}/$${arch}/kubectl"; \
	echo "Downloading kubectl $(KUBECTL_VERSION) from $$url"; \
	curl -L -o "$(KUBECTL)" "$$url"; \
	chmod +x "$(KUBECTL)"; \
	echo "✅ kubectl installed at $(KUBECTL)"

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

### CUSTOM TARGETS ###
ensuremockoauth2isreachable: kubefwd ## Ensure kubefwd is installed and running and that mock-oauth2 is reachable
	@KUBEFWD_PIDS=$$(pgrep -f "kubefwd( |$$)" 2>/dev/null); \
	if [ -z "$$KUBEFWD_PIDS" ]; then \
		echo -e "❌  mock-oauth2 ingress is not configured."; \
		echo -e "    Configure it with 'make mock-oauth2-ingress'"; \
		exit 1; \
	fi
	@echo "Verifying mock-oauth2 is reachable via kubefwd..."
	@HTTP_STATUS=$$(curl -s -o /dev/null -w "%{http_code}" http://mock-oauth2:8080/accesserator/.well-known/openid-configuration); \
	if [ "$$HTTP_STATUS" = "200" ]; then \
		echo -e "✅  mock-oauth2 is reachable (HTTP 200)"; \
	else \
		echo -e "❌  mock-oauth2 returned HTTP $$HTTP_STATUS (expected 200)."; \
		echo -e "    Make sure mock-oauth2 is running and that forwarding is configured."; \
		echo -e "    This can be done with 'make mock-oauth2' and 'make mock-oauth2-ingress' respectively."; \
		exit 1; \
	fi

create-namespace: kubectl
	$(if $(strip $(namespace)),,$(error namespace is not set))
	@echo "🤞 Creating namespace: $(namespace)"
	@output=$$($(KUBECTL) create namespace "$(namespace)" --context "$(KUBECONTEXT)" 2>&1); \
	status=$$?; \
	if [ $$status -eq 0 ]; then \
		echo "✅ Namespace '$(namespace)' created successfully"; \
	elif echo "$$output" | grep -qiE "already exists|AlreadyExists"; then \
		echo "✅ Namespace '$(namespace)' already exists, continuing..."; \
	else \
		echo "❌ Error creating '$(namespace)' namespace:"; \
		echo "$$output"; \
		exit 1; \
	fi
	$(KUBECTL) label namespaces $(namespace) istio.io/rev=default

wait-for-skiperator-pod: kubectl
	$(if $(strip $(app)),,$(error app is not set))
	$(if $(strip $(namespace)),,$(error namespace is not set))
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/wait-for-skiperator-pod-ready.sh $(app) $(namespace)

