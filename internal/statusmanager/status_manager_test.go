package statusmanager_test

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/internal/statusmanager"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const synchronizationStateNotReady = "not ready"

func newTestSecurityConfigForStatusManager() *v1alpha.SecurityConfig {
	return &v1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-security-config",
			Namespace:  "default",
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityConfig",
			APIVersion: "accesserator.kartverket.no/v1alpha",
		},
		Spec: v1alpha.SecurityConfigSpec{
			ApplicationRef: "my-app",
		},
		Status: v1alpha.SecurityConfigStatus{
			Phase:              v1alpha.PhaseReady,
			Ready:              true,
			Message:            "SecurityConfig ready.",
			ObservedGeneration: 1,
		},
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = v1alpha.AddToScheme(scheme)
	return scheme
}

var _ = Describe("DetermineReconciliationState", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		k8sClient = fake.NewClientBuilder().WithScheme(newScheme()).Build()
	})

	It("returns StateInvalid when config is invalid", func() {
		scope := &state.Scope{
			InvalidConfig:          true,
			ValidationErrorMessage: utilities.Ptr("Invalid configuration"),
			Descendants:            []state.Descendant[client.Object]{},
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StateInvalid))
	})

	It("returns StatePending when descendants count does not match reconciled resources", func() {
		scope := &state.Scope{
			InvalidConfig: false,
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-oauth-secret", Object: &corev1.Secret{}},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
			newMockResource("NetworkPolicy", "my-netpol", false),
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StatePending))
	})

	It("returns StateFailed when descendants have errors", func() {
		errorMsg := "Failed to create resource"
		scope := &state.Scope{
			InvalidConfig: false,
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-oauth-secret", Object: &corev1.Secret{}, ErrorMessage: &errorMsg},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StateFailed))
	})

	It("returns StateReady when config is valid, all descendants present, and no errors", func() {
		successMsg := "Created successfully"
		scope := &state.Scope{
			InvalidConfig: false,
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-oauth-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StateReady))
	})

	Context("when TokenX is enabled", func() {
		It("returns StateWaitingForJwker when Jwker is not yet ready", func() {
			jwker := newTestJwker("default", "my-app", synchronizationStateNotReady, "")
			k8sClient = newK8sClientWithObjects(jwker)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
				},
				TokenXConfig:  state.TokenXConfig{Enabled: true},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateWaitingForJwker))
		})

		It("returns StateReady and sets JwkerSecretName when Jwker is ready", func() {
			const secretName = "my-app-jwker-secret"
			jwker := newTestJwker("default", "my-app", utilities.JwkerSynchronizationStateReady, secretName)
			k8sClient = newK8sClientWithObjects(jwker)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
				},
				TokenXConfig:  state.TokenXConfig{Enabled: true},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateReady))
			Expect(scope.SecurityConfig.Status.JwkerSecretName).To(Equal(secretName),
				"JwkerSecretName should be set from the Jwker synchronization secret name")
		})

		It("returns an error when the Jwker resource cannot be found", func() {
			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
				},
				TokenXConfig:  state.TokenXConfig{Enabled: true},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get Jwker resource"))
			Expect(result).To(BeNil())
		})
	})

	Context("when Maskinporten is enabled through inlineClient or clientRef", func() {
		It("returns StateWaitingForMaskinportenClient when MaskinportenClient is not yet ready", func() {
			maskinportenClient := newTestMaskinportenClient("default", "my-app", synchronizationStateNotReady, "")
			k8sClient = newK8sClientWithObjects(maskinportenClient)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
				},
				MaskinportenConfig: state.MaskinportenConfig{
					Enabled: true,
					Type:    state.InlineClient,
				},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateWaitingForMaskinportenClient))
		})

		It("returns StateReady and sets MaskinportenClientSecretName when MaskinportenClient is ready", func() {
			const secretName = "my-app-maskinporten-secret"
			maskinportenClient := newTestMaskinportenClient("default", "my-app", utilities.MaskinportenClientSynchronizationStateReady, secretName)
			k8sClient = newK8sClientWithObjects(maskinportenClient)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec: v1alpha.SecurityConfigSpec{
						ApplicationRef: "my-app",
						Maskinporten: &v1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &v1alpha.ResourceRef{
								Name: v1alpha.ResourceName(utilities.GetMaskinportenClientName("my-app")),
							},
						},
					},
				},
				MaskinportenConfig: state.MaskinportenConfig{
					Enabled: true,
					Type:    state.ClientRef,
				},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateReady))
			Expect(scope.SecurityConfig.Status.MaskinportenSecretName).To(Equal(secretName),
				"MaskinportenClientSecretName should be set from the MaskinportenClient synchronization secret name")
		})

		It("returns an error when the MaskinportenClient resource cannot be found", func() {
			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
				},
				MaskinportenConfig: state.MaskinportenConfig{
					Enabled: true,
					Type:    state.InlineClient,
				},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get MaskinportenClient resource"))
			Expect(result).To(BeNil())
		})
	})

	It("returns StateReady when Maskinporten is enabled through secretRef", func() {
		scope := &state.Scope{
			SecurityConfig: v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
				Spec:       v1alpha.SecurityConfigSpec{ApplicationRef: "my-app"},
			},
			MaskinportenConfig: state.MaskinportenConfig{
				Enabled: true,
				Type:    state.SecretRef,
			},
			InvalidConfig: false,
			Descendants:   []state.Descendant[client.Object]{},
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StateReady))
		Expect(scope.SecurityConfig.Status.MaskinportenSecretName).To(Equal(utilities.GetMaskinportenSecretFromSecretRefName(scope.SecurityConfig.Name)),
			"MaskinportenClientSecretName should be set from utilities.GetMaskinportenSecretFromSecretRefName func")
	})

	Context("when both TokenX and Maskinporten are enabled", func() {
		It("returns StateWaitingForMaskinportenClient when Jwker is ready but MaskinportenClient is not ready", func() {
			jwker := newTestJwker("default", "my-app", utilities.JwkerSynchronizationStateReady, "my-app-jwker-secret")
			maskinportenClient := newTestMaskinportenClient("default", "my-app", synchronizationStateNotReady, "")
			k8sClient = newK8sClientWithObjects(jwker, maskinportenClient)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec: v1alpha.SecurityConfigSpec{
						ApplicationRef: "my-app",
						Maskinporten: &v1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &v1alpha.ResourceRef{
								Name: v1alpha.ResourceName(utilities.GetMaskinportenClientName("my-app")),
							},
						},
					},
				},
				TokenXConfig: state.TokenXConfig{Enabled: true},
				MaskinportenConfig: state.MaskinportenConfig{
					Enabled: true,
					Type:    state.ClientRef,
				},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateWaitingForMaskinportenClient))
		})

		It("returns StateWaitingForJwker when MaskinportenClient is ready but Jwker is not ready", func() {
			jwker := newTestJwker("default", "my-app", synchronizationStateNotReady, "")
			maskinportenClient := newTestMaskinportenClient("default", "my-app", utilities.MaskinportenClientSynchronizationStateReady, "mp-secret")
			k8sClient = newK8sClientWithObjects(jwker, maskinportenClient)

			scope := &state.Scope{
				SecurityConfig: v1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "sc", Namespace: "default"},
					Spec: v1alpha.SecurityConfigSpec{
						ApplicationRef: "my-app",
						Maskinporten: &v1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &v1alpha.ResourceRef{
								Name: v1alpha.ResourceName(utilities.GetMaskinportenClientName("my-app")),
							},
						},
					},
				},
				TokenXConfig: state.TokenXConfig{Enabled: true},
				MaskinportenConfig: state.MaskinportenConfig{
					Enabled: true,
					Type:    state.ClientRef,
				},
				InvalidConfig: false,
				Descendants:   []state.Descendant[client.Object]{},
			}

			result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, []reconciliation.ControllerResource{})

			Expect(err).NotTo(HaveOccurred())
			Expect(*result).To(Equal(statusmanager.StateWaitingForJwker))
		})
	})

	It("StateInvalid takes precedence over StatePending and StateFailed", func() {
		errorMsg := "Some error"
		scope := &state.Scope{
			InvalidConfig:          true,
			ValidationErrorMessage: utilities.Ptr("Invalid configuration"),
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-oauth-secret", Object: &corev1.Secret{}, ErrorMessage: &errorMsg},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
			newMockResource("NetworkPolicy", "my-netpol", false),
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StateInvalid), "Invalid config should take precedence over other states")
	})

	It("StatePending takes precedence over StateFailed", func() {
		errorMsg := "Some error"
		scope := &state.Scope{
			InvalidConfig: false,
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-oauth-secret", Object: &corev1.Secret{}, ErrorMessage: &errorMsg},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
			newMockResource("NetworkPolicy", "my-netpol", false),
		}

		result, err := statusmanager.DetermineReconciliationState(ctx, k8sClient, scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*result).To(Equal(statusmanager.StatePending), "Pending state should take precedence over failed state")
	})
})

var _ = Describe("UpdateSecurityConfigStatus", func() {
	var (
		ctx          context.Context
		k8sClient    client.Client
		sc           *v1alpha.SecurityConfig
		fakeRecorder *events.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		sc = newTestSecurityConfigForStatusManager()
		fakeRecorder = events.NewFakeRecorder(10)
	})

	Context("when OPA is enabled", func() {
		It("sets OpaBundleSource from bundle data in sorted order", func() {
			sc := newTestSecurityConfigForStatusManager()
			original := sc.DeepCopy()

			k8sClient = fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(sc).
				WithStatusSubresource(sc).
				Build()

			scope := &state.Scope{
				SecurityConfig: *sc,
				OpaConfig: state.OpaConfig{
					Enabled: true,
					BundleBinaryData: map[string][]byte{
						"bundle-b": []byte("data-b"),
						"bundle-a": []byte("data-a"),
					},
				},
			}

			statusmanager.UpdateSecurityConfigStatus(ctx, k8sClient, fakeRecorder, scope, original, []reconciliation.ControllerResource{})

			updated := &v1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sc), updated)).To(Succeed())
			Expect(updated.Status.OpaBundleSource.ConfigMapName).To(Equal(utilities.GetOpaConfigMapName(scope.SecurityConfig.Name)))
			Expect(updated.Status.OpaBundleSource.BundleNames).To(Equal([]string{"bundle-a", "bundle-b"}))
		})
	})

	It("does not emit StatusUpdateSuccess when status is unchanged", func() {
		sc.Status.Conditions = []metav1.Condition{
			{
				Type:               "SecurityConfig-test-security-config",
				Status:             metav1.ConditionTrue,
				Reason:             "ReconciliationSuccess",
				Message:            "Descendants of SecurityConfig reconciled successfully.",
				LastTransitionTime: metav1.Now(),
			},
		}
		original := sc.DeepCopy()

		k8sClient = fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(sc).
			WithStatusSubresource(sc).
			Build()

		scope := &state.Scope{
			SecurityConfig: *sc,
			InvalidConfig:  false,
			Descendants:    []state.Descendant[client.Object]{},
		}

		statusmanager.UpdateSecurityConfigStatus(ctx, k8sClient, fakeRecorder, scope, original, []reconciliation.ControllerResource{})

		close(fakeRecorder.Events)
		recordedEvents := make([]string, 0, len(fakeRecorder.Events))
		for event := range fakeRecorder.Events {
			recordedEvents = append(recordedEvents, event)
		}

		Expect(recordedEvents).To(HaveLen(1))
		Expect(recordedEvents[0]).To(ContainSubstring("StatusUpdateStarted"))
		Expect(recordedEvents[0]).NotTo(ContainSubstring("StatusUpdateSuccess"))
	})

	It("emits StatusUpdateSuccess and updates status when status changes", func() {
		sc.Status.Phase = v1alpha.PhasePending
		sc.Status.Ready = false
		original := sc.DeepCopy()

		k8sClient = fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(sc).
			WithStatusSubresource(sc).
			Build()

		successMsg := "Created"
		scope := &state.Scope{
			SecurityConfig: *sc,
			InvalidConfig:  false,
			Descendants: []state.Descendant[client.Object]{
				{ID: "Secret-test", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "test", false),
		}

		statusmanager.UpdateSecurityConfigStatus(ctx, k8sClient, fakeRecorder, scope, original, resources)

		close(fakeRecorder.Events)
		recordedEvents := make([]string, 0, len(fakeRecorder.Events))
		for event := range fakeRecorder.Events {
			recordedEvents = append(recordedEvents, event)
		}

		Expect(recordedEvents).To(HaveLen(2))
		Expect(recordedEvents[0]).To(ContainSubstring("Normal StatusUpdateStarted"))
		Expect(recordedEvents[1]).To(ContainSubstring("Normal StatusUpdateSuccess"))

		updated := &v1alpha.SecurityConfig{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sc), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha.PhaseReady))
		Expect(updated.Status.Ready).To(BeTrue())
	})

	It("emits StatusUpdateFailed when the status update fails", func() {
		sc.Status.Phase = v1alpha.PhasePending
		original := sc.DeepCopy()

		// Client without the SecurityConfig to trigger a NotFound update failure
		k8sClient = fake.NewClientBuilder().
			WithScheme(newScheme()).
			Build()

		scope := &state.Scope{
			SecurityConfig: *sc,
			InvalidConfig:  false,
			Descendants:    []state.Descendant[client.Object]{},
		}

		statusmanager.UpdateSecurityConfigStatus(ctx, k8sClient, fakeRecorder, scope, original, []reconciliation.ControllerResource{})

		close(fakeRecorder.Events)
		recordedEvents := make([]string, 0, len(fakeRecorder.Events))
		for event := range fakeRecorder.Events {
			recordedEvents = append(recordedEvents, event)
		}

		Expect(recordedEvents).To(HaveLen(2))
		Expect(recordedEvents[0]).To(ContainSubstring("Normal StatusUpdateStarted"))
		Expect(recordedEvents[1]).To(ContainSubstring("Warning StatusUpdateFailed"))
	})
})
