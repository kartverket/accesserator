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

var _ = Describe("ConfigMapShouldUpdateFunc", func() {
	It("returns false when current and desired are equal", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
			Data:       map[string]string{"key": "value"},
			Immutable:  utilities.Ptr(false),
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
			Data:       map[string]string{"key": "value"},
			Immutable:  utilities.Ptr(false),
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeFalse())
	})

	It("returns false when both are empty", func() {
		Expect(reconciler.ConfigMapShouldUpdateFunc(&corev1.ConfigMap{}, &corev1.ConfigMap{})).To(BeFalse())
	})

	It("returns true when a binary data value changes", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("2")},
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a binary data key is added", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{
				"a": []byte("1"),
				"b": []byte("1"),
			},
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a binary data key is removed", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{
				"a": []byte("1"),
				"b": []byte("1"),
			},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when current binary data is empty and desired is not", func() {
		current := &corev1.ConfigMap{
			BinaryData: map[string][]byte{},
		}
		desired := &corev1.ConfigMap{
			BinaryData: map[string][]byte{"a": []byte("1")},
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Data differs", func() {
		current := &corev1.ConfigMap{
			Data: map[string]string{"key": "old"},
		}
		desired := &corev1.ConfigMap{
			Data: map[string]string{"key": "new"},
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Immutable differs", func() {
		current := &corev1.ConfigMap{
			Immutable: utilities.Ptr(false),
		}
		desired := &corev1.ConfigMap{
			Immutable: utilities.Ptr(true),
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Immutable is set on desired but not on current", func() {
		current := &corev1.ConfigMap{}
		desired := &corev1.ConfigMap{
			Immutable: utilities.Ptr(true),
		}

		Expect(reconciler.ConfigMapShouldUpdateFunc(current, desired)).To(BeTrue())
	})
})

var _ = Describe("SecretShouldUpdateFunc", func() {
	It("returns false when current and desired are equal", func() {
		current := &corev1.Secret{
			StringData: map[string]string{"key": "value"},
			Data:       map[string][]byte{"a": []byte("1")},
			Immutable:  utilities.Ptr(false),
		}
		desired := &corev1.Secret{
			StringData: map[string]string{"key": "value"},
			Data:       map[string][]byte{"a": []byte("1")},
			Immutable:  utilities.Ptr(false),
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeFalse())
	})

	It("returns false when both are empty", func() {
		Expect(reconciler.SecretShouldUpdateFunc(&corev1.Secret{}, &corev1.Secret{})).To(BeFalse())
	})

	It("returns true when a Data value changes", func() {
		current := &corev1.Secret{
			Data: map[string][]byte{"token": []byte("old")},
		}
		desired := &corev1.Secret{
			Data: map[string][]byte{"token": []byte("new")},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a Data key is added", func() {
		current := &corev1.Secret{
			Data: map[string][]byte{"token": []byte("a")},
		}
		desired := &corev1.Secret{
			Data: map[string][]byte{
				"token": []byte("a"),
				"extra": []byte("b"),
			},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a Data key is removed", func() {
		current := &corev1.Secret{
			Data: map[string][]byte{
				"token": []byte("a"),
				"extra": []byte("b"),
			},
		}
		desired := &corev1.Secret{
			Data: map[string][]byte{"token": []byte("a")},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when current Data is empty and desired is not", func() {
		current := &corev1.Secret{
			Data: map[string][]byte{},
		}
		desired := &corev1.Secret{
			Data: map[string][]byte{"token": []byte("a")},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when StringData differs", func() {
		current := &corev1.Secret{
			StringData: map[string]string{"key": "old"},
		}
		desired := &corev1.Secret{
			StringData: map[string]string{"key": "new"},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Immutable differs", func() {
		current := &corev1.Secret{
			Immutable: utilities.Ptr(false),
		}
		desired := &corev1.Secret{
			Immutable: utilities.Ptr(true),
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Immutable is set on desired but not on current", func() {
		current := &corev1.Secret{}
		desired := &corev1.Secret{
			Immutable: utilities.Ptr(true),
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("ignores Type differences", func() {
		current := &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
		}
		desired := &corev1.Secret{
			Type: corev1.SecretTypeBasicAuth,
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeFalse())
	})
})
