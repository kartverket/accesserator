package reconciler_test

import (
	"fmt"
	"time"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/reconciler"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ControllerResources", func() {
	var (
		testNamespace  string
		securityConfig accesseratorv1alpha.SecurityConfig
	)

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-resources-%d", time.Now().UnixNano())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		securityConfig = accesseratorv1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: testNamespace,
			},
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				ApplicationRef: "my-app",
			},
		}
	})

	AfterEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	Context("when InvalidConfig is true", func() {
		It("returns nil", func() {
			scope := &state.Scope{
				SecurityConfig: securityConfig,
				InvalidConfig:  true,
			}
			Expect(reconciler.ControllerResources(scope)).To(BeNil())
		})
	})

	Context("when InvalidConfig is false", func() {
		var scope *state.Scope

		BeforeEach(func() {
			scope = &state.Scope{
				SecurityConfig: securityConfig,
				InvalidConfig:  false,
			}
		})

		It("returns resources with correct kinds", func() {
			resources := reconciler.ControllerResources(scope)
			kinds := make([]string, len(resources))
			for i, r := range resources {
				kinds[i] = r.GetResourceKind()
			}
			Expect(kinds).To(
				ConsistOf(
					"Jwker",
					"NetworkPolicy",
					"MaskinportenClient",
					"Secret",
					"ServiceEntry",
					"ConfigMap",
				),
			)
		})

		It("returns resources with correct names", func() {
			resources := reconciler.ControllerResources(scope)
			namesByKind := make(map[string]string, len(resources))
			for _, r := range resources {
				namesByKind[r.GetResourceKind()] = r.GetResourceName()
			}

			Expect(namesByKind["Jwker"]).To(Equal(
				utilities.GetJwkerName(string(securityConfig.Spec.ApplicationRef)),
			))
			Expect(namesByKind["NetworkPolicy"]).To(Equal(
				utilities.GetTokenxEgressName(scope.SecurityConfig.Name, config.Get().TokenxName),
			))
			Expect(namesByKind["MaskinportenClient"]).To(Equal(
				utilities.GetMaskinportenClientName(string(securityConfig.Spec.ApplicationRef)),
			))
			Expect(namesByKind["Secret"]).To(Equal(
				utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name),
			))
			Expect(namesByKind["ServiceEntry"]).To(Equal(
				utilities.GetMaskinportenServiceEntryName(securityConfig.Name),
			))
		})
	})
})

var _ = Describe("maskinportenSecretControllerResource", func() {
	var (
		testNamespace  string
		securityConfig accesseratorv1alpha.SecurityConfig
		scope          *state.Scope
		adapter        reconciler.ControllerResourceAdapter[*corev1.Secret]
	)

	getAdapter := func() reconciler.ControllerResourceAdapter[*corev1.Secret] {
		var a reconciler.ControllerResourceAdapter[*corev1.Secret]
		for _, r := range reconciler.ControllerResources(scope) {
			if candidate, ok := r.(reconciler.ControllerResourceAdapter[*corev1.Secret]); ok {
				a = candidate
				break
			}
		}
		Expect(a.Func.ShouldUpdate).NotTo(BeNil())
		return a
	}

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-secret-resources-%d", time.Now().UnixNano())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		securityConfig = accesseratorv1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: testNamespace,
				UID:       "test-uid-secret",
			},
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				ApplicationRef: "my-app",
			},
		}

		secretData := map[string][]byte{
			"token": []byte("initial-token"),
		}
		scope = &state.Scope{
			SecurityConfig: securityConfig,
			MaskinportenConfig: state.MaskinportenConfig{
				Enabled:    true,
				Type:       state.SecretRef,
				SecretData: &secretData,
			},
		}

		adapter = getAdapter()
	})

	AfterEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = k8sClient.Delete(ctx, ns)
	})

	It("creates a Secret when it does not exist", func() {
		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		createdSecret := &corev1.Secret{}
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      adapter.GetResourceName(),
			Namespace: testNamespace,
		}, createdSecret)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdSecret.Data).To(HaveKey("token"))
	})

	It("updates a Secret when data changes", func() {
		secretName := utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name)

		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"token": []byte("old-token"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		newData := map[string][]byte{
			"token": []byte("new-token"),
		}
		scope.MaskinportenConfig.SecretData = &newData
		adapter = getAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Data["token"]).To(Equal([]byte("new-token")))
	})

	It("does not update a Secret when data is unchanged", func() {
		secretName := utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name)

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		before := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, before)).To(Succeed())
		rvBefore := before.ResourceVersion

		_, err = adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		after := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, after)).To(Succeed())
		Expect(after.ResourceVersion).To(Equal(rvBefore))
	})

	It("updates a Secret when a new data key is added", func() {
		secretName := utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name)

		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"token": []byte("a-token"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		newData := map[string][]byte{
			"token": []byte("a-token"),
			"extra": []byte("extra-value"),
		}
		scope.MaskinportenConfig.SecretData = &newData
		adapter = getAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Data).To(HaveKey("extra"))
	})

	It("updates a Secret when a data key is removed", func() {
		secretName := utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name)

		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"token": []byte("a-token"),
				"extra": []byte("extra-value"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		newData := map[string][]byte{
			"token": []byte("a-token"),
		}
		scope.MaskinportenConfig.SecretData = &newData
		adapter = getAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Data).NotTo(HaveKey("extra"))
	})
})

var _ = Describe("opaConfigMapControllerResource", func() {
	var adapter reconciler.ControllerResourceAdapter[*corev1.ConfigMap]

	BeforeEach(func() {
		scope := &state.Scope{
			SecurityConfig: accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "test-ns",
				},
			},
			OpaConfig: state.OpaConfig{
				Enabled:          true,
				BundleBinaryData: map[string][]byte{"bundle": []byte("data")},
			},
		}

		for _, r := range reconciler.ControllerResources(scope) {
			if cmAdapter, ok := r.(reconciler.ControllerResourceAdapter[*corev1.ConfigMap]); ok {
				adapter = cmAdapter
				break
			}
		}
		Expect(adapter.Func.ShouldUpdate).NotTo(BeNil())
	})

	It("returns false when binary data is identical", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}

		Expect(adapter.Func.ShouldUpdate(current, desired)).To(BeFalse())
	})

	It("returns true when a value changes", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("2")},
		}

		Expect(adapter.Func.ShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a new key is added", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{
				"a": []byte("1"),
				"b": []byte("1"),
			},
		}

		Expect(adapter.Func.ShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a key is removed", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{
				"a": []byte("1"),
				"b": []byte("1"),
			},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}

		Expect(adapter.Func.ShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when lengths differ because current data is empty", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}

		Expect(adapter.Func.ShouldUpdate(current, desired)).To(BeTrue())
	})
})
