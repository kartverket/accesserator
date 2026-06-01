package utilities_test

import (
	"fmt"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func securityConfig() v1alpha.SecurityConfig {
	return v1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-name"},
		Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: v1alpha.ResourceName("my-ref")},
	}
}

var _ = Describe("Helper Functions", func() {

	Describe("Ptr", func() {
		It("should return a pointer to the value", func() {
			v := 42
			ptr := utilities.Ptr(v)
			Expect(ptr).NotTo(BeNil())
			Expect(*ptr).To(Equal(v))
		})

		It("should work with different types", func() {
			strVal := "hello"
			strPtr := utilities.Ptr(strVal)
			Expect(strPtr).NotTo(BeNil())
			Expect(*strPtr).To(Equal(strVal))
			boolPtr := utilities.Ptr(true)
			Expect(boolPtr).NotTo(BeNil())
			Expect(*boolPtr).To(BeTrue())
		})
	})

	Describe("SecurityConfigStandardLabels", func() {
		It("returns the standard Accesserator labels with the resource name", func() {
			Expect(utilities.SecurityConfigStandardLabels()).To(Equal(map[string]string{
				"app.kubernetes.io/managed-by":          "accesserator",
				"accesserator.kartverket.no/controller": "securityconfig",
			}))
		})
	})

	Describe("LowestNonZeroResult", func() {
		var (
			zero ctrl.Result
			one  ctrl.Result
			two  ctrl.Result
		)

		BeforeEach(func() {
			zero = ctrl.Result{}
			one = ctrl.Result{RequeueAfter: 1 * time.Second}
			two = ctrl.Result{RequeueAfter: 2 * time.Second}
		})

		It("should return zero result when both are zero", func() {
			result := utilities.LowestNonZeroResult(zero, zero)
			Expect(result).To(Equal(zero))
		})

		It("should return non-zero result when first is zero", func() {
			result := utilities.LowestNonZeroResult(zero, one)
			Expect(result).To(Equal(one))
		})

		It("should return non-zero result when second is zero", func() {
			result := utilities.LowestNonZeroResult(one, zero)
			Expect(result).To(Equal(one))
		})

		It("should return the lower non-zero result", func() {
			result := utilities.LowestNonZeroResult(one, two)
			Expect(result).To(Equal(one))

			result = utilities.LowestNonZeroResult(two, one)
			Expect(result).To(Equal(one))
		})
	})

	Describe("TokenxNamer", func() {
		namer := utilities.NewTokenxNamer(securityConfig())

		It("JwkerName returns the base", func() {
			Expect(namer.JwkerName()).
				To(Equal("my-ref"))
		})

		It("SecretName returns base with jwker-secret suffix", func() {
			Expect(namer.SecretName()).
				To(Equal("my-ref-jwker-secret"))
		})

		It("EgressName returns base with tokenx name and egress suffix", func() {
			Expect(namer.EgressName("tokenx")).
				To(Equal("my-name-tokenx-egress"))
		})
	})

	Describe("MaskinportenNamer", func() {
		namer := utilities.NewMaskinportenNamer(securityConfig())

		It("MaskinportenClientName returns the base", func() {
			Expect(namer.MaskinportenClientName()).
				To(Equal("my-ref"))
		})

		It("SecretName returns base with maskinporten suffix", func() {
			Expect(namer.SecretName()).
				To(Equal("my-ref-maskinporten-c593fd9d"))
		})

		It("SecretFromRefName returns base with maskinporten suffix and hash", func() {
			Expect(namer.SecretFromRefName()).
				To(Equal("my-name-maskinporten-5f817fb7"))
		})

		It("ServiceEntryName returns base with maskinporten suffix", func() {
			Expect(namer.ServiceEntryName()).
				To(Equal("my-name-maskinporten-5f817fb7"))
		})
	})

	Describe("EntraIdNamer", func() {
		namer := utilities.NewEntraIdNamer(securityConfig())

		It("AzureAdApplicationName returns the base", func() {
			Expect(namer.AzureAdApplicationName()).
				To(Equal("my-ref"))
		})

		It("SecretName returns base with entraid suffix", func() {
			Expect(namer.SecretName()).
				To(Equal("my-ref-entraid-a2dceed7"))
		})

		It("SecretFromRefName returns base with entraid suffix and hash", func() {
			Expect(namer.SecretFromRefName()).
				To(Equal("my-name-entraid-848ad57d"))
		})

		It("ServiceEntryName returns base with entraid suffix", func() {
			Expect(namer.ServiceEntryName()).
				To(Equal("my-name-entraid-848ad57d"))
		})
	})

	Describe("OpaNamer", func() {
		It("ConfigMapName returns base with opa suffix", func() {
			namer := utilities.NewOpaNamer(securityConfig())
			Expect(namer.ConfigMapName()).
				To(Equal("my-name-opa-bc75e7d6"))
		})
	})

	Describe("WithShortHashSuffix", func() {
		It("should append the short hash as suffix", func() {
			prefix := "my-prefix"
			suffix := utilities.ShortHash(prefix)
			expected := fmt.Sprintf("%s-%s", prefix, suffix)
			Expect(utilities.WithShortHashSuffix(prefix)).To(Equal(expected))
		})
	})

	Describe("ShortHash", func() {
		It("should return an 8-character hex string", func() {
			result := utilities.ShortHash("test")
			Expect(result).To(HaveLen(8))
			Expect(result).To(MatchRegexp("^[0-9a-f]{8}$"))
		})

		It("should return the same hash for the same input", func() {
			input := "my-security-config"
			result1 := utilities.ShortHash(input)
			result2 := utilities.ShortHash(input)
			Expect(result1).To(Equal(result2))
		})

		It("should return different hashes for different inputs", func() {
			result1 := utilities.ShortHash("input1")
			result2 := utilities.ShortHash("input2")
			Expect(result1).NotTo(Equal(result2))
		})
	})

	Describe("GetMockKubernetesClient", func() {
		It("should return a non-nil client", func() {
			scheme := runtime.NewScheme()
			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion("v1")
			obj.SetKind("ConfigMap")
			obj.SetName("test-cm")
			client := utilities.GetMockKubernetesClient(scheme, obj)
			Expect(client).NotTo(BeNil())
		})

		It("should work with empty objects", func() {
			scheme := runtime.NewScheme()
			client := utilities.GetMockKubernetesClient(scheme)
			Expect(client).NotTo(BeNil())
		})
	})

	type ComplexObject struct {
		Foo v1alpha.ResourceName
		Bar int32
		Baz string
	}

	Describe("UniqueSliceElements", func() {
		It("should return unique string elements", func() {
			input := []string{"foo", "bar", "foo", "baz"}
			expect := []string{"foo", "bar", "baz"}
			result := utilities.UniqueSliceElements(input)
			Expect(result).To(HaveLen(3))
			Expect(result).To(Equal(expect))
		})

		It("should return unique complex elements", func() {
			equal1 := ComplexObject{
				Foo: "foo",
				Bar: 100,
				Baz: "baz",
			}
			equal2 := ComplexObject{
				Foo: "foo",
				Bar: 100,
				Baz: "baz",
			}
			notequal1 := ComplexObject{
				Foo: "foo",
				Bar: 200,
				Baz: "baz",
			}
			notequal2 := ComplexObject{
				Foo: "notfoo",
				Bar: 100,
				Baz: "baz",
			}
			notequal3 := ComplexObject{
				Foo: "foo",
				Bar: 100,
				Baz: "notbaz",
			}
			result1 := utilities.UniqueSliceElements([]ComplexObject{equal1, equal2})
			Expect(result1).To(HaveLen(1))
			result2 := utilities.UniqueSliceElements([]ComplexObject{equal1, equal2, notequal1})
			Expect(result2).To(HaveLen(2))
			result3 := utilities.UniqueSliceElements([]ComplexObject{equal1, equal2, notequal2})
			Expect(result3).To(HaveLen(2))
			result4 := utilities.UniqueSliceElements([]ComplexObject{equal1, equal2, notequal3})
			Expect(result4).To(HaveLen(2))
			result5 := utilities.UniqueSliceElements([]ComplexObject{equal1, equal2, notequal1, notequal2, notequal3})
			Expect(result5).To(HaveLen(4))
		})
	})
})
