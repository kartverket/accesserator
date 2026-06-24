package eventhandler_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func idportenWithConfigMapAudience(name, namespace, configMapName string) *v1alpha.SecurityConfig {
	return &v1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha.SecurityConfigSpec{
			ApplicationRef: "app-a",
			Idporten: &v1alpha.IdPortenSpec{
				Enabled: true,
				AllowedAudiences: []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: "AUDIENCE"},
					}},
				},
			},
		},
	}
}

var _ = Describe("HandleConfigMapEvent", func() {
	It("enqueues SecurityConfigs in the same namespace referencing the configmap as an ID-porten audience source", func() {
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared-configmap", Namespace: "team-a"}}

		scMatch := idportenWithConfigMapAudience("sc-match", "team-a", "shared-configmap")
		scNoMatch := idportenWithConfigMapAudience("sc-no-match", "team-a", "other-configmap")
		scWrongNamespace := idportenWithConfigMapAudience("sc-wrong-ns", "team-b", "shared-configmap")

		// References the same name but via a Secret ref, so the ConfigMap handler must not match it.
		scSecretRefSameName := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-secret-ref", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Idporten: &v1alpha.IdPortenSpec{
					Enabled: true,
					AllowedAudiences: []v1alpha.AllowedAudience{
						{ValueFrom: &v1alpha.ValueFrom{
							SecretKeyRef: &v1alpha.KeyRef{Name: "shared-configmap", Key: "AUDIENCE"},
						}},
					},
				},
			},
		}

		// ID-porten not configured at all.
		scNoIdporten := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-no-idporten", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten:   &v1alpha.MaskinportenSpec{Enabled: true},
			},
		}

		c := buildClient(scMatch, scNoMatch, scWrongNamespace, scSecretRefSameName, scNoIdporten)
		h := eventhandler.HandleConfigMapEvent(c)

		requests := runCreateEvent(h, configMap)

		Expect(requests).To(ConsistOf(req("team-a", "sc-match")))
	})

	It("returns no requests for unrelated object type", func() {
		c := buildClient()
		h := eventhandler.HandleConfigMapEvent(c)

		requests := runCreateEvent(h, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"}})

		Expect(requests).To(BeEmpty())
	})
})
