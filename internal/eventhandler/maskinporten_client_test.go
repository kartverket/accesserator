package eventhandler_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HandleMaskinportenClientEvent", func() {
	It("enqueues SecurityConfigs in the same namespace referencing the MaskinportenClient", func() {
		clientObj := &naisiov1.MaskinportenClient{ObjectMeta: metav1.ObjectMeta{Name: "mp-client", Namespace: "team-a"}}

		scMatch1 := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-match-1", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					ClientRef: &v1alpha.ResourceRef{Name: "mp-client"},
				},
			},
		}
		scMatch2 := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-match-2", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					ClientRef: &v1alpha.ResourceRef{Name: "mp-client"},
				},
			},
		}
		scNoMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-no-match", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					ClientRef: &v1alpha.ResourceRef{Name: "other-client"},
				},
			},
		}
		scWrongNamespace := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-wrong-ns", Namespace: "team-b"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					ClientRef: &v1alpha.ResourceRef{Name: "mp-client"},
				},
			},
		}

		c := buildClient(scMatch1, scMatch2, scNoMatch, scWrongNamespace)
		h := eventhandler.HandleMaskinportenClientEvent(c)

		requests := runCreateEvent(h, clientObj)

		Expect(requests).To(ConsistOf(
			req("team-a", "sc-match-1"),
			req("team-a", "sc-match-2"),
		))
	})

	It("returns no requests for unrelated object type", func() {
		c := buildClient()
		h := eventhandler.HandleMaskinportenClientEvent(c)

		requests := runCreateEvent(h, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}})

		Expect(requests).To(BeEmpty())
	})
})
