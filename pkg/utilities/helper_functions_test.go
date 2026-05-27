package utilities_test

import (
	"fmt"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Helper Functions", func() {
	const (
		secConfigName = "my-security-config"
	)

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

	Describe("GetJwkerName", func() {
		It("should return the application ref as the jwker name", func() {
			appRef := "my-app"
			result := utilities.GetJwkerName(appRef)
			Expect(result).To(Equal(appRef))
		})
	})

	Describe("GetJwkerSecretName", func() {
		It("should return jwker name with secret suffix", func() {
			jwkerName := "foo"
			expected := fmt.Sprintf("%s-%s", jwkerName, utilities.JwkerSecretNameSuffix)
			result := utilities.GetJwkerSecretName(jwkerName)
			Expect(result).To(Equal(expected))
		})
	})

	Describe("GetTokenxEgressName", func() {
		It("should return combined name with egress suffix", func() {
			secName := "sec"
			tokenx := "tok"
			expected := fmt.Sprintf("%s-%s-%s", secName, tokenx, utilities.EgressNameSuffix)
			result := utilities.GetTokenxEgressName(secName, tokenx)
			Expect(result).To(Equal(expected))
		})
	})

	Describe("GetMaskinportenClientName", func() {
		It("should return the application ref as the maskinportenclient name", func() {
			appRef := "my-app"
			result := utilities.GetMaskinportenClientName(appRef)
			Expect(result).To(Equal(appRef))
		})
	})

	Describe("GetMaskinportenSecretName", func() {
		It("should return security config name with maskinporten suffix", func() {
			expected := fmt.Sprintf("%s-%s", secConfigName, utilities.MaskinportenNameSuffix)
			result := utilities.GetMaskinportenSecretName(secConfigName)
			Expect(result).To(Equal(expected))
		})
	})

	Describe("GetMaskinportenSecretFromSecretRefName", func() {
		It("should return security config name with maskinporten suffix and hash", func() {
			expectedHash := utilities.ShortHash(secConfigName)
			expected := fmt.Sprintf("%s-%s-%s", secConfigName, utilities.MaskinportenNameSuffix, expectedHash)
			result := utilities.GetMaskinportenSecretFromSecretRefName(secConfigName)
			Expect(result).To(Equal(expected))
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

	Describe("GetMaskinportenServiceEntryName", func() {
		It("should return security config name with maskinporten suffix", func() {
			expected := fmt.Sprintf("%s-%s", secConfigName, utilities.MaskinportenNameSuffix)
			result := utilities.GetMaskinportenServiceEntryName(secConfigName)
			Expect(result).To(Equal(expected))
		})
	})

	Describe("GetOpaConfigMapName", func() {
		It("should return security config name with OPA config map suffix", func() {
			expected := fmt.Sprintf("%s-%s", secConfigName, utilities.OpaConfigMapNameSuffix)
			result := utilities.GetOpaConfigMapName(secConfigName)
			Expect(result).To(Equal(expected))
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
