package eventhandler_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	skiperatorv1alpha1 "github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HandleSkiperatorApplicationEvent", func() {
	It("enqueues only SecurityConfigs in the same namespace referencing the application", func() {
		app := &skiperatorv1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app-a", Namespace: "team-a"}}

		scMatch1 := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-match-1", Namespace: "team-a"},
			Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "app-a"},
		}
		scMatch2 := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-match-2", Namespace: "team-a"},
			Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "app-a"},
		}
		scWrongRef := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-wrong-ref", Namespace: "team-a"},
			Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "app-b"},
		}
		scWrongNamespace := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-wrong-ns", Namespace: "team-b"},
			Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "app-a"},
		}

		c := buildClient(scMatch1, scMatch2, scWrongRef, scWrongNamespace)
		h := eventhandler.HandleSkiperatorApplicationEvent(c)

		requests := runCreateEvent(h, app)

		Expect(requests).To(ConsistOf(
			req("team-a", "sc-match-1"),
			req("team-a", "sc-match-2"),
		))
	})

	It("returns no requests for unrelated object type", func() {
		c := buildClient()
		h := eventhandler.HandleSkiperatorApplicationEvent(c)

		requests := runCreateEvent(h, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}})

		Expect(requests).To(BeEmpty())
	})
})
