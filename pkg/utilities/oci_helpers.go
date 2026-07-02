package utilities

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type OciRepositoryAndDigest struct {
	Repository *remote.Repository
	Digest     string
}

func ResolveOciRepositoryAndDigest(
	ctx context.Context,
	credStore credentials.Store,
	ociReference string,
) (*OciRepositoryAndDigest, error) {
	var ociRepoAndDigest OciRepositoryAndDigest
	repo, err := remote.NewRepository(ociReference)
	if err != nil {
		return nil, fmt.Errorf("failed parsing OCI reference %q: %w", ociReference, err)
	}
	repo.Client = &auth.Client{
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}
	ociRepoAndDigest.Repository = repo
	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve OCI reference %q: %w", ociReference, err)
	}
	ociRepoAndDigest.Digest = desc.Digest.String()
	return &ociRepoAndDigest, nil
}

// FetchLayerMatchingMediaType fetches the layer matching the provided media type
// from the manifest at the given digest.
func FetchLayerMatchingMediaType(
	ctx context.Context,
	ociRepoAndDigest OciRepositoryAndDigest,
	mediaType string,
) ([]byte, error) {
	manifest, err := fetchManifest(ctx, ociRepoAndDigest)
	if err != nil {
		return nil, err
	}

	for _, layer := range manifest.Layers {
		if layer.MediaType != mediaType {
			continue
		}
		blob, fetchLayerErr := content.FetchAll(ctx, ociRepoAndDigest.Repository.Blobs(), layer)
		if fetchLayerErr != nil {
			return nil, fmt.Errorf(
				"failed to fetch layer of mediaType %s from %s: %w",
				mediaType,
				layer.Digest,
				fetchLayerErr,
			)
		}
		return blob, nil
	}

	return nil, fmt.Errorf(
		"no layer of mediaType %s found in manifest %s",
		mediaType,
		ociRepoAndDigest.Digest,
	)
}

// GetManifestDigestWithoutAlgPrefix turns "sha256:<hex>" into the raw 32-byte hash.
func GetManifestDigestWithoutAlgPrefix(manifestDigest string) ([]byte, error) {
	algo, hexPart, ok := strings.Cut(manifestDigest, ":")
	if !ok || algo != "sha256" {
		return nil, fmt.Errorf("unsupported manifest digest %q (only sha256 supported)", manifestDigest)
	}
	raw, err := hex.DecodeString(hexPart)
	if err != nil {
		return nil, fmt.Errorf("error when decoding manifest digest hex: %w", err)
	}
	return raw, nil
}

func fetchManifest(ctx context.Context, ociRepoAndDigest OciRepositoryAndDigest) (*ocispec.Manifest, error) {
	_, reader, err := ociRepoAndDigest.
		Repository.
		Manifests().
		FetchReference(ctx, ociRepoAndDigest.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OCI manifest %s: %w", ociRepoAndDigest.Digest, err)
	}
	defer func(reader io.ReadCloser) {
		closeErr := reader.Close()
		if closeErr != nil {
			panic("failed to close OCI manifest reader for " + ociRepoAndDigest.Digest + ": " + closeErr.Error())
		}
	}(reader)

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCI manifest %s: %w", ociRepoAndDigest.Digest, err)
	}
	var manifest ocispec.Manifest
	if unmarshalErr := json.Unmarshal(bytes, &manifest); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal OCI manifest %s: %w", ociRepoAndDigest.Digest, unmarshalErr)
	}
	return &manifest, nil
}

func GetOciReferrersMatchingMediaTypeAndPredicate(
	ctx context.Context,
	ociRepoAndDigest OciRepositoryAndDigest,
	mediaType string,
	predicate func(referrer ocispec.Descriptor) bool,
) ([]ocispec.Descriptor, error) {
	subject := ocispec.Descriptor{Digest: digest.Digest(ociRepoAndDigest.Digest)}
	var matchingReferrers []ocispec.Descriptor
	err := ociRepoAndDigest.Repository.Referrers(
		ctx,
		subject,
		mediaType,
		func(referrers []ocispec.Descriptor) error {
			for _, referrer := range referrers {
				if predicate(referrer) {
					matchingReferrers = append(matchingReferrers, referrer)
				}
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list OCI 1.1 referrers for %s: %w", ociRepoAndDigest.Digest, err)
	}

	if len(matchingReferrers) == 0 {
		return nil, fmt.Errorf("no OCI 1.1 referrer found for %s", ociRepoAndDigest.Digest)
	}
	return matchingReferrers, nil
}
