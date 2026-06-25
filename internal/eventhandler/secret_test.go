package eventhandler_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HandleSecretEvent", func() {
	It("enqueues SecurityConfigs in the same namespace referencing the secret in clientID or clientJWK", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shared-secret", Namespace: "team-a"}}

		scClientIDMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-client-id", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "shared-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "other-secret", Key: "jwk"},
					},
				},
			},
		}
		scClientJWKMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-client-jwk", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "other-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "shared-secret", Key: "jwk"},
					},
				},
			},
		}
		scNoMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-no-match", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "another-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "another-secret", Key: "jwk"},
					},
				},
			},
		}
		scWrongNamespace := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-wrong-ns", Namespace: "team-b"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Maskinporten: &v1alpha.MaskinportenSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "shared-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "shared-secret", Key: "jwk"},
					},
				},
			},
		}

		c := buildClient(scClientIDMatch, scClientJWKMatch, scNoMatch, scWrongNamespace)
		h := eventhandler.HandleSecretEvent(c)

		requests := runCreateEvent(h, secret)

		Expect(requests).To(ConsistOf(
			req("team-a", "sc-client-id"),
			req("team-a", "sc-client-jwk"),
		))
	})

	It("enqueues SecurityConfigs in the same namespace referencing the secret in entraid clientID or clientJWK", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shared-secret", Namespace: "team-a"}}

		scClientIDMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-entraid-client-id", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				EntraID: &v1alpha.EntraIDSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "shared-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "other-secret", Key: "jwk"},
					},
				},
			},
		}
		scClientJWKMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-entraid-client-jwk", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				EntraID: &v1alpha.EntraIDSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "other-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "shared-secret", Key: "jwk"},
					},
				},
			},
		}
		scNoMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-entraid-no-match", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				EntraID: &v1alpha.EntraIDSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "another-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "another-secret", Key: "jwk"},
					},
				},
			},
		}
		scWrongNamespace := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-entraid-wrong-ns", Namespace: "team-b"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				EntraID: &v1alpha.EntraIDSpec{
					SecretRef: &v1alpha.SecretRef{
						ClientID:  v1alpha.SecretKeySelector{Name: "shared-secret", Key: "id"},
						ClientJWK: v1alpha.SecretKeySelector{Name: "shared-secret", Key: "jwk"},
					},
				},
			},
		}

		c := buildClient(scClientIDMatch, scClientJWKMatch, scNoMatch, scWrongNamespace)
		h := eventhandler.HandleSecretEvent(c)

		requests := runCreateEvent(h, secret)

		Expect(requests).To(ConsistOf(
			req("team-a", "sc-entraid-client-id"),
			req("team-a", "sc-entraid-client-jwk"),
		))
	})

	It("enqueues SecurityConfigs referencing the secret as an ID-porten audience source", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "audience-secret", Namespace: "team-a"}}

		scIdportenMatch := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-idporten", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Idporten: &v1alpha.IdPortenSpec{
					Enabled: true,
					AllowedAudience: v1alpha.AllowedAudience{
						ValueFrom: &v1alpha.ValueFrom{
							SecretKeyRef: &v1alpha.KeyRef{Name: "audience-secret", Key: "AUDIENCE"},
						},
					},
				},
			},
		}
		// References the same name but via a ConfigMap ref, so the Secret handler must not match it.
		scIdportenConfigMapRefSameName := &v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "sc-idporten-cm", Namespace: "team-a"},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app-a",
				Idporten: &v1alpha.IdPortenSpec{
					Enabled: true,
					AllowedAudience: v1alpha.AllowedAudience{
						ValueFrom: &v1alpha.ValueFrom{
							ConfigMapKeyRef: &v1alpha.KeyRef{Name: "audience-secret", Key: "AUDIENCE"},
						},
					},
				},
			},
		}

		c := buildClient(scIdportenMatch, scIdportenConfigMapRefSameName)
		h := eventhandler.HandleSecretEvent(c)

		requests := runCreateEvent(h, secret)

		Expect(requests).To(ConsistOf(req("team-a", "sc-idporten")))
	})

	It("returns no requests for unrelated object type", func() {
		c := buildClient()
		h := eventhandler.HandleSecretEvent(c)

		requests := runCreateEvent(h, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}})

		Expect(requests).To(BeEmpty())
	})
})
