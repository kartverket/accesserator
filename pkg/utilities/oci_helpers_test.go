package utilities_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

// fakeRegistry is a minimal in-memory OCI distribution registry served over
// plain HTTP via httptest. It serves manifests, blobs and the Referrers API,
// which is all the OCI helpers under test exercise.
type fakeRegistry struct {
	server    *httptest.Server
	manifests map[string]storedContent // keyed by digest
	blobs     map[string][]byte        // keyed by digest
	referrers map[string][]ocispec.Descriptor
}

type storedContent struct {
	mediaType string
	bytes     []byte
}

const fakeRepoName = "test/repo"

func newFakeRegistry() *fakeRegistry {
	reg := &fakeRegistry{
		manifests: map[string]storedContent{},
		blobs:     map[string][]byte{},
		referrers: map[string][]ocispec.Descriptor{},
	}
	reg.server = httptest.NewServer(http.HandlerFunc(reg.handle))
	return reg
}

func (reg *fakeRegistry) close() { reg.server.Close() }

// repository returns a plain-HTTP repository pointing at this fake registry.
func (reg *fakeRegistry) repository() *remote.Repository {
	u, err := url.Parse(reg.server.URL)
	Expect(err).NotTo(HaveOccurred())
	repo, err := remote.NewRepository(u.Host + "/" + fakeRepoName)
	Expect(err).NotTo(HaveOccurred())
	repo.PlainHTTP = true
	return repo
}

// addManifest stores a manifest and returns its descriptor.
func (reg *fakeRegistry) addManifest(manifest ocispec.Manifest) ocispec.Descriptor {
	if manifest.MediaType == "" {
		manifest.MediaType = ocispec.MediaTypeImageManifest
	}
	raw, err := json.Marshal(manifest)
	Expect(err).NotTo(HaveOccurred())
	dig := digest.FromBytes(raw).String()
	reg.manifests[dig] = storedContent{mediaType: manifest.MediaType, bytes: raw}
	return ocispec.Descriptor{MediaType: manifest.MediaType, Digest: digest.Digest(dig), Size: int64(len(raw))}
}

// addBlob stores a blob and returns its descriptor.
func (reg *fakeRegistry) addBlob(mediaType string, data []byte) ocispec.Descriptor {
	dig := digest.FromBytes(data).String()
	reg.blobs[dig] = data
	return ocispec.Descriptor{MediaType: mediaType, Digest: digest.Digest(dig), Size: int64(len(data))}
}

func (reg *fakeRegistry) handle(w http.ResponseWriter, r *http.Request) {
	prefix := "/v2/" + fakeRepoName + "/"
	switch {
	case r.URL.Path == "/v2/" || r.URL.Path == "/v2":
		w.WriteHeader(http.StatusOK)
	case strings.HasPrefix(r.URL.Path, prefix+"manifests/"):
		ref := strings.TrimPrefix(r.URL.Path, prefix+"manifests/")
		stored, ok := reg.manifests[ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", stored.mediaType)
		w.Header().Set("Docker-Content-Digest", ref)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(stored.bytes)
	case strings.HasPrefix(r.URL.Path, prefix+"blobs/"):
		ref := strings.TrimPrefix(r.URL.Path, prefix+"blobs/")
		data, ok := reg.blobs[ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Docker-Content-Digest", ref)
		_, _ = w.Write(data)
	case strings.HasPrefix(r.URL.Path, prefix+"referrers/"):
		ref := strings.TrimPrefix(r.URL.Path, prefix+"referrers/")
		index := ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: reg.referrers[ref],
		}
		raw, err := json.Marshal(index)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
		_, _ = w.Write(raw)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

var _ = Describe("GetManifestDigestWithoutAlgPrefix", func() {
	It("decodes an all-zero sha256 digest into 32 zero bytes", func() {
		got, err := utilities.StripAlgPrefix(
			"sha256:0000000000000000000000000000000000000000000000000000000000000000",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(32))
		for _, b := range got {
			Expect(b).To(BeEquivalentTo(0))
		}
	})

	It("decodes a non-zero sha256 digest into the expected bytes", func() {
		got, err := utilities.StripAlgPrefix(
			"sha256:8f3a1b9c2d4e5f600102030405060708090a0b0c0d0e0f10111213141516170a",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(32))
		Expect(got[0]).To(BeEquivalentTo(0x8f))
		Expect(got[31]).To(BeEquivalentTo(0x0a))
	})

	It("returns an error for a digest with a non-sha256 algorithm prefix", func() {
		_, err := utilities.StripAlgPrefix("sha512:" + strings.Repeat("a", 128))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only sha256 supported"))
	})

	It("returns an error for a digest with no algorithm separator", func() {
		_, err := utilities.StripAlgPrefix(strings.Repeat("a", 64))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only sha256 supported"))
	})

	It("returns an error when the hex portion contains invalid characters", func() {
		_, err := utilities.StripAlgPrefix("sha256:zz" + strings.Repeat("0", 62))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error when decoding manifest digest hex"))
	})
})

var _ = Describe("ResolveOciRepositoryAndDigest", func() {
	It("returns an error when the reference is empty", func() {
		_, err := utilities.ResolveOciRepositoryAndDigest(context.Background(), nil, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed parsing OCI reference"))
	})

	It("returns an error when the reference is malformed", func() {
		_, err := utilities.ResolveOciRepositoryAndDigest(context.Background(), nil, "not::a::valid::reference")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed parsing OCI reference"))
	})

	It("returns an error when the registry cannot be reached", func() {
		// 127.0.0.1:1 is a well-formed reference but nothing listens there,
		// so resolving the reference fails without any external network access.
		_, err := utilities.ResolveOciRepositoryAndDigest(
			context.Background(),
			nil,
			"127.0.0.1:1/some/repo:tag",
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to resolve OCI reference"))
	})
})

var _ = Describe("FetchLayerMatchingMediaType", func() {
	const layerMediaType = "application/vnd.test.layer.v1+tar"

	var (
		registry     *fakeRegistry
		repoAndDig   utilities.OciRepositoryAndDigest
		layerContent = []byte("the-layer-bytes")
	)

	BeforeEach(func() {
		registry = newFakeRegistry()
		layerDesc := registry.addBlob(layerMediaType, layerContent)
		configDesc := registry.addBlob("application/vnd.oci.image.config.v1+json", []byte("{}"))
		manifestDesc := registry.addManifest(ocispec.Manifest{
			Config: configDesc,
			Layers: []ocispec.Descriptor{layerDesc},
		})
		repoAndDig = utilities.OciRepositoryAndDigest{
			Repository: registry.repository(),
			Digest:     manifestDesc.Digest.String(),
		}
	})

	AfterEach(func() {
		registry.close()
	})

	It("returns the layer whose media type matches", func() {
		blob, err := utilities.FetchLayerMatchingMediaType(context.Background(), repoAndDig, layerMediaType)
		Expect(err).NotTo(HaveOccurred())
		Expect(blob).To(Equal(layerContent))
	})

	It("returns an error when no layer matches the media type", func() {
		_, err := utilities.FetchLayerMatchingMediaType(
			context.Background(),
			repoAndDig,
			"application/vnd.does.not.exist",
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no layer of mediaType"))
	})

	It("returns an error when the manifest cannot be fetched", func() {
		missing := utilities.OciRepositoryAndDigest{
			Repository: registry.repository(),
			Digest:     "sha256:" + strings.Repeat("0", 64),
		}
		_, err := utilities.FetchLayerMatchingMediaType(context.Background(), missing, layerMediaType)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to fetch OCI manifest"))
	})
})

var _ = Describe("GetOciReferrersMatchingMediaTypeAndPredicate", func() {
	const artifactType = "application/vnd.test.attestation"

	var (
		reg          *fakeRegistry
		repoAndDig   utilities.OciRepositoryAndDigest
		matching     ocispec.Descriptor
		manifestDesc ocispec.Descriptor
	)

	BeforeEach(func() {
		reg = newFakeRegistry()
		configDesc := reg.addBlob("application/vnd.oci.image.config.v1+json", []byte("{}"))
		manifestDesc = reg.addManifest(ocispec.Manifest{Config: configDesc})

		matching = ocispec.Descriptor{
			MediaType:    ocispec.MediaTypeImageManifest,
			ArtifactType: artifactType,
			Digest:       digest.FromString("matching"),
			Size:         1,
			Annotations:  map[string]string{"keep": "yes"},
		}
		nonMatching := ocispec.Descriptor{
			MediaType:    ocispec.MediaTypeImageManifest,
			ArtifactType: artifactType,
			Digest:       digest.FromString("non-matching"),
			Size:         1,
			Annotations:  map[string]string{"keep": "no"},
		}
		reg.referrers[manifestDesc.Digest.String()] = []ocispec.Descriptor{matching, nonMatching}

		repoAndDig = utilities.OciRepositoryAndDigest{
			Repository: reg.repository(),
			Digest:     manifestDesc.Digest.String(),
		}
	})

	AfterEach(func() {
		reg.close()
	})

	It("returns only the referrers satisfying the predicate", func() {
		got, err := utilities.GetOciReferrersMatchingMediaTypeAndPredicate(
			context.Background(),
			repoAndDig,
			artifactType,
			func(referrer ocispec.Descriptor) bool {
				return referrer.Annotations["keep"] == "yes"
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(1))
		Expect(got[0].Digest).To(Equal(matching.Digest))
	})

	It("returns an error when no referrer satisfies the predicate", func() {
		_, err := utilities.GetOciReferrersMatchingMediaTypeAndPredicate(
			context.Background(),
			repoAndDig,
			artifactType,
			func(ocispec.Descriptor) bool { return false },
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no OCI 1.1 referrer found"))
	})

	It("returns an error when there are no referrers at all", func() {
		empty := utilities.OciRepositoryAndDigest{
			Repository: reg.repository(),
			Digest:     "sha256:" + strings.Repeat("a", 64),
		}
		_, err := utilities.GetOciReferrersMatchingMediaTypeAndPredicate(
			context.Background(),
			empty,
			artifactType,
			func(ocispec.Descriptor) bool { return true },
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no OCI 1.1 referrer found"))
	})
})
