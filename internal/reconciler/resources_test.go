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
	istioapiv1 "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
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
					"AzureAdApplication",
					"Secret",
					"ServiceEntry",
					"ConfigMap",
				),
			)
		})

		It("returns resources with correct names", func() {
			resources := reconciler.ControllerResources(scope)

			resourceKindsAndNames := make([]string, len(resources))
			for i, r := range resources {
				resourceKindsAndNames[i] = fmt.Sprintf("%s/%s", r.GetResourceKind(), r.GetResourceName())
			}

			Expect(resourceKindsAndNames).To(
				ConsistOf(
					fmt.Sprintf("%s/%s", "Jwker", utilities.NewTokenxNamer(securityConfig).JwkerName()),
					fmt.Sprintf("%s/%s", "NetworkPolicy", utilities.NewTokenxNamer(securityConfig).EgressName(config.Get().TokenxName)),
					fmt.Sprintf("%s/%s", "MaskinportenClient", utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName()),
					fmt.Sprintf("%s/%s", "Secret", utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()),
					fmt.Sprintf("%s/%s", "ServiceEntry", utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName()),
					fmt.Sprintf("%s/%s", "AzureAdApplication", utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName()),
					fmt.Sprintf("%s/%s", "Secret", utilities.NewEntraIdNamer(securityConfig).SecretFromRefName()),
					fmt.Sprintf("%s/%s", "ServiceEntry", utilities.NewEntraIdNamer(securityConfig).ServiceEntryName()),
					fmt.Sprintf("%s/%s", "ConfigMap", utilities.NewOpaNamer(securityConfig).ConfigMapName()),
				),
			)
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
		Expect(createdSecret.Labels).To(SatisfyAll(
			HaveKeyWithValue("app.kubernetes.io/managed-by", "accesserator"),
			HaveKeyWithValue("accesserator.kartverket.no/controller", "securityconfig"),
		))
	})

	It("updates a Secret when data changes", func() {
		secretName := utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()

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

	It("adds standard labels to an existing Secret that is missing them", func() {
		secretName := utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()

		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"token": []byte("initial-token"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Labels).To(SatisfyAll(
			HaveKeyWithValue("app.kubernetes.io/managed-by", "accesserator"),
			HaveKeyWithValue("accesserator.kartverket.no/controller", "securityconfig"),
		))
	})

	It("does not update a Secret when data is unchanged", func() {
		secretName := utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()

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
		secretName := utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()

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
		secretName := utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName()

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

	It("returns true when a desired label is missing on current", func() {
		current := &corev1.ConfigMap{}
		desired := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "accesserator"},
			},
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

	It("returns true when a desired label is missing on current", func() {
		current := &corev1.Secret{}
		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "accesserator"},
			},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label has a different value on current", func() {
		current := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "someone-else"},
			},
		}
		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "accesserator"},
			},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	// The "returns true" tests for each resource type already ensured the label clause is included in the ShouldUpdate
	// func. We still have to test the "returns false" case, but only for one (randomly chosen) resource type.
	It("returns false when desired labels are a subset of current labels", func() {
		current := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "accesserator",
					"custom.example.com/team":      "platform",
				},
			},
		}
		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "accesserator"},
			},
		}

		Expect(reconciler.SecretShouldUpdateFunc(current, desired)).To(BeFalse())
	})
})

var _ = Describe("ServiceEntryShouldUpdateFunc", func() {
	It("returns false when current and desired are equal", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				ExportTo:   []string{".", "istio-system"},
				Hosts:      []string{"example.com"},
				Ports:      []*istioapiv1.ServicePort{{Name: "https", Number: 443, Protocol: "HTTPS"}},
				Resolution: istioapiv1.ServiceEntry_DNS,
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				ExportTo:   []string{".", "istio-system"},
				Hosts:      []string{"example.com"},
				Ports:      []*istioapiv1.ServicePort{{Name: "https", Number: 443, Protocol: "HTTPS"}},
				Resolution: istioapiv1.ServiceEntry_DNS,
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeFalse())
	})

	It("returns false when both are empty", func() {
		current := &istionetworkingv1.ServiceEntry{}
		desired := &istionetworkingv1.ServiceEntry{}
		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeFalse())
	})

	It("returns true when ExportTo differs", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				ExportTo: []string{"."},
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				ExportTo: []string{".", "istio-system"},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Hosts differs", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Hosts: []string{"old.example.com"},
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Hosts: []string{"new.example.com"},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a Port is added", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{{Name: "https", Number: 443, Protocol: "HTTPS"}},
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{
					{Name: "https", Number: 443, Protocol: "HTTPS"},
					{Name: "http", Number: 80, Protocol: "HTTP"},
				},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a Port is removed", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{
					{Name: "https", Number: 443, Protocol: "HTTPS"},
					{Name: "http", Number: 80, Protocol: "HTTP"},
				},
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{
					{Name: "https", Number: 443, Protocol: "HTTPS"},
				},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a Port value changes", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{{Name: "https", Number: 443, Protocol: "HTTPS"}},
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Ports: []*istioapiv1.ServicePort{{Name: "https", Number: 8443, Protocol: "HTTPS"}},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when Resolution differs", func() {
		current := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Resolution: istioapiv1.ServiceEntry_DNS,
			},
		}
		desired := &istionetworkingv1.ServiceEntry{
			Spec: istioapiv1.ServiceEntry{
				Resolution: istioapiv1.ServiceEntry_STATIC,
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label is missing on current", func() {
		current := &istionetworkingv1.ServiceEntry{}
		desired := &istionetworkingv1.ServiceEntry{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app.kubernetes.io/managed-by": "accesserator"},
			},
		}

		Expect(reconciler.ServiceEntryShouldUpdateFunc(current, desired)).To(BeTrue())
	})
})
