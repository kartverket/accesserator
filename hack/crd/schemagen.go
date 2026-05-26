//go:build generate

package crd

// Create an alias for the tool command to download CRD manifests.
// This runs the downloader program located at hack/url2crd.
//go:generate -command urlcrd go run ../../hack/url2crd

// Skiperator Application CRD
//go:generate urlcrd -outdir=./bases -url=https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/crd/skiperator.kartverket.no_applications.yaml

// Jwker CRD
//go:generate urlcrd -outdir=./bases -url=https://raw.githubusercontent.com/nais/liberator/f638cfb830180dc1d175cab8e7a07a1606688667/config/crd/bases/nais.io_jwkers.yaml

// MaskinportenClient CRD
//go:generate urlcrd -outdir=./bases -url=https://raw.githubusercontent.com/nais/liberator/f638cfb830180dc1d175cab8e7a07a1606688667/config/crd/bases/nais.io_maskinportenclients.yaml

// AzureAdApplication CRD
//go:generate urlcrd -outdir=./bases -url=https://raw.githubusercontent.com/nais/liberator/f638cfb830180dc1d175cab8e7a07a1606688667/config/crd/bases/nais.io_azureadapplications.yaml

// Istio ServiceEntry CRD
//go:generate urlcrd -outdir=./bases -kind=ServiceEntry -url=https://raw.githubusercontent.com/istio/api/refs/tags/1.28.0/kubernetes/customresourcedefinitions.gen.yaml
