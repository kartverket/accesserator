package controller_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/kartverket/accesserator/internal/controller"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/labels"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

var _ = Describe("SecurityConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			securityConfigName = "test-resource"
			skiperatorAppName  = "test-app"
			namespaceName      = "default"
		)

		ctx = context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      securityConfigName,
			Namespace: namespaceName,
		}
		securityConfig := accesseratorv1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{Name: securityConfigName},
			Spec:       accesseratorv1alpha.SecurityConfigSpec{ApplicationRef: skiperatorAppName},
		}
		application := &v1alpha1.Application{}

		BeforeEach(func() {
			By("creating the dependent Application custom resource")
			appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: "default"}
			err := k8sClient.Get(ctx, appKey, application)
			if err != nil && errors.IsNotFound(err) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      skiperatorAppName,
						Namespace: namespaceName,
					},
					Spec: v1alpha1.ApplicationSpec{
						AccessPolicy: &podtypes.AccessPolicy{},
					},
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind SecurityConfig")
			err = k8sClient.Get(ctx, typeNamespacedName, &securityConfig)
			if err != nil && errors.IsNotFound(err) {
				securityConfig := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      securityConfigName,
						Namespace: namespaceName,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: skiperatorAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
					},
				}
				Expect(k8sClient.Create(ctx, securityConfig)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			resource := &accesseratorv1alpha.SecurityConfig{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SecurityConfig")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Cleanup any created Jwker resource")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{Name: utilities.NewTokenxNamer(securityConfig).JwkerName(), Namespace: namespaceName}
			if err := k8sClient.Get(ctx, jwkerKey, jwker); err == nil {
				Expect(k8sClient.Delete(ctx, jwker)).To(Succeed())
			}

			By("Cleanup any created Netpol resource")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{Name: utilities.NewTokenxNamer(securityConfig).EgressName(config.Get().TokenxName), Namespace: namespaceName}
			if err := k8sClient.Get(ctx, netpolKey, netpol); err == nil {
				Expect(k8sClient.Delete(ctx, netpol)).To(Succeed())
			}

			By("Cleanup any created MaskinportenClient resource")
			maskinportenClientCleanup := &naisiov1.MaskinportenClient{}
			maskinportenClientCleanupKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, maskinportenClientCleanupKey, maskinportenClientCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, maskinportenClientCleanup)).To(Succeed())
			}

			By("Cleanup any created maskinporten ServiceEntry resource")
			maskinportenServiceEntryCleanup := &istionetworkingv1.ServiceEntry{}
			maskinportenServiceEntryCleanupKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, maskinportenServiceEntryCleanupKey, maskinportenServiceEntryCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, maskinportenServiceEntryCleanup)).To(Succeed())
			}

			By("Cleanup any created maskinporten integration Secret resource")
			maskinportenSecretCleanup := &corev1.Secret{}
			maskinportenSecretCleanupKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, maskinportenSecretCleanupKey, maskinportenSecretCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, maskinportenSecretCleanup)).To(Succeed())
			}

			By("Cleanup any created AzureAdApplication resource")
			azureAdApplicationCleanup := &naisiov1.AzureAdApplication{}
			azureAdApplicationCleanupKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, azureAdApplicationCleanupKey, azureAdApplicationCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, azureAdApplicationCleanup)).To(Succeed())
			}

			By("Cleanup any created AzureAd ServiceEntry resource")
			azureAdServiceEntryCleanup := &istionetworkingv1.ServiceEntry{}
			azureAdServiceEntryCleanupKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, azureAdServiceEntryCleanupKey, azureAdServiceEntryCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, azureAdServiceEntryCleanup)).To(Succeed())
			}

			By("Cleanup any created azuread integration Secret resource")
			azureAdSecretCleanup := &corev1.Secret{}
			azureAdSecretCleanupKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, azureAdSecretCleanupKey, azureAdSecretCleanup); err == nil {
				Expect(k8sClient.Delete(ctx, azureAdSecretCleanup)).To(Succeed())
			}

			skiperatorApp := &v1alpha1.Application{}
			appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: namespaceName}
			err = k8sClient.Get(ctx, appKey, skiperatorApp)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the dependant Application resource")
			Expect(k8sClient.Delete(ctx, skiperatorApp)).To(Succeed())
		})

		It("should create a Jwker resource and a NetworkPolicy when TokenX is enabled", func() {
			By("Reconciling the SecurityConfig with TokenX enabled")

			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a NetworkPolicy resource was created")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{
				Name:      utilities.NewTokenxNamer(securityConfig).EgressName(config.Get().TokenxName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, netpolKey, netpol)
			}).Should(Succeed())

			By("Verifying that a Jwker resource was created")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{
				Name:      utilities.NewTokenxNamer(securityConfig).JwkerName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, jwkerKey, jwker)
			}).Should(Succeed())

			By("Verifying that SecurityConfig is PhasePending before Jwker is ready")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, sc); err != nil {
					return "", err
				}
				return sc.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the Jwker resource as finished reconciling")
			jwker.Status.SynchronizationState = utilities.JwkerSynchronizationStateReady
			Expect(k8sClient.Status().Update(ctx, jwker)).To(Succeed())

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, sc); err != nil {
					return "", err
				}
				return sc.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))

			By("Verifying events were emitted")
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileStarted")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconciledSuccessfully")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileSuccess")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("StatusUpdateSuccess")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("StatusUpdateFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
		})

		It("should create MaskinportenClient and ServiceEntry (NOT Secret) when Maskinporten is enabled via inline client, and SecurityConfig should have correct status", func() {
			By("Updating SecurityConfig with Maskinporten inline client configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					Client: &accesseratorv1alpha.MaskinportenClientSpec{
						ClientName: "test-client",
						Scopes: &accesseratorv1alpha.MaskinportenScope{
							ConsumedScopes: []naisiov1.ConsumedScope{
								{
									Name: "random-scope:read",
								},
							},
						},
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with Maskinporten inline client enabled")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a MaskinportenClient resource was created")
			maskinportenClient := &naisiov1.MaskinportenClient{}
			maskinportenClientKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, maskinportenClientKey, maskinportenClient)
			}).Should(Succeed())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that a Secret was NOT created")
			secret := &corev1.Secret{}
			secretKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, secretKey, secret))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhasePending while waiting for MaskinportenClient")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the MaskinportenClient resource as ready")
			maskinportenClient.Status.SynchronizationState = utilities.MaskinportenClientSynchronizationStateReady
			maskinportenClient.Status.SynchronizationSecretName = utilities.NewMaskinportenNamer(securityConfig).SecretName()
			Expect(k8sClient.Status().Update(ctx, maskinportenClient)).To(Succeed())

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should ONLY create ServiceEntry (NOT Secret or MaskinportenClient) when Maskinporten is enabled via clientRef, and SecurityConfig should have correct status", func() {
			const externalClientName = "external-maskinporten-client"

			By("Pre-creating a MaskinportenClient to be referenced via clientRef")
			externalClient := &naisiov1.MaskinportenClient{
				ObjectMeta: metav1.ObjectMeta{
					Name:      externalClientName,
					Namespace: namespaceName,
				},
				Spec: naisiov1.MaskinportenClientSpec{
					SecretName: "external-maskinporten-secret",
				},
			}
			Expect(k8sClient.Create(ctx, externalClient)).To(Succeed())
			DeferCleanup(func() {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: externalClientName, Namespace: namespaceName}, externalClient); err == nil {
					Expect(k8sClient.Delete(ctx, externalClient)).To(Succeed())
				}
			})

			By("Updating SecurityConfig with Maskinporten clientRef configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					ClientRef: &accesseratorv1alpha.ResourceRef{
						Name: externalClientName,
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with Maskinporten clientRef")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that no new MaskinportenClient was created by the controller")
			controllerCreatedClient := &naisiov1.MaskinportenClient{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
					Namespace: namespaceName,
				}, controllerCreatedClient))
			}).Should(BeTrue())

			By("Verifying that no Secret was created")
			secret := &corev1.Secret{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName(),
					Namespace: namespaceName,
				}, secret))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhasePending while waiting for the referenced MaskinportenClient")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the referenced MaskinportenClient as ready")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: externalClientName, Namespace: namespaceName}, externalClient)).To(Succeed())
			externalClient.Status.SynchronizationState = utilities.MaskinportenClientSynchronizationStateReady
			externalClient.Status.SynchronizationSecretName = "external-maskinporten-secret"
			Expect(k8sClient.Status().Update(ctx, externalClient)).To(Succeed())

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should ONLY create a ServiceEntry and Secret (NOT MaskinportenClient) when Maskinporten is enabled via secretRef, and SecurityConfig should have correct status", func() {
			const (
				sourceSecretName = "maskinporten-credentials"
				clientIDKey      = "clientId"
				clientJWKKey     = "clientJwk"
			)

			By("Pre-creating a Kubernetes Secret containing Maskinporten credentials")
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceSecretName,
					Namespace: namespaceName,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					clientIDKey:  []byte("test-client-id"),
					clientJWKKey: []byte(`{"kty":"RSA","use":"sig","alg":"RS256","kid":"test"}`),
				},
			}
			Expect(k8sClient.Create(ctx, sourceSecret)).To(Succeed())
			DeferCleanup(func() {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceSecretName, Namespace: namespaceName}, sourceSecret); err == nil {
					Expect(k8sClient.Delete(ctx, sourceSecret)).To(Succeed())
				}
			})

			By("Updating SecurityConfig with Maskinporten secretRef configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					SecretRef: &accesseratorv1alpha.SecretRef{
						ClientID: accesseratorv1alpha.SecretKeySelector{
							Name: sourceSecretName,
							Key:  clientIDKey,
						},
						ClientJWK: accesseratorv1alpha.SecretKeySelector{
							Name: sourceSecretName,
							Key:  clientJWKKey,
						},
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with Maskinporten secretRef")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that an integration Secret was created")
			integrationSecret := &corev1.Secret{}
			integrationSecretKey := types.NamespacedName{
				Name:      utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, integrationSecretKey, integrationSecret)
			}).Should(Succeed())

			By("Verifying that no MaskinportenClient was created")
			maskinportenClient := &naisiov1.MaskinportenClient{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
					Namespace: namespaceName,
				}, maskinportenClient))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhaseReady (secretRef does not require waiting)")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should create AzureAdApplication and ServiceEntry (NOT Secret) when Entraid is enabled via inline client, and SecurityConfig should have correct status", func() {
			By("Updating SecurityConfig with EntraID inline client configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				EntraID: &accesseratorv1alpha.EntraIDSpec{
					Enabled: true,
					Client: &accesseratorv1alpha.AzureAdApplicationSpec{
						SecretName: "test-client",
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with EntraID inline client enabled")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a AzureAdApplication resource was created")
			azureAdApplication := &naisiov1.AzureAdApplication{}
			azureAdApplicationKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, azureAdApplicationKey, azureAdApplication)
			}).Should(Succeed())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that a Secret was NOT created")
			secret := &corev1.Secret{}
			secretKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, secretKey, secret))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhasePending while waiting for AzureAdApplication")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the AzureAdApplication resource as ready")
			azureAdApplication.Status.SynchronizationState = utilities.AzureAdApplicationSynchronizationStateReady
			azureAdApplication.Status.SynchronizationSecretName = utilities.NewEntraIdNamer(securityConfig).SecretName()
			Expect(k8sClient.Status().Update(ctx, azureAdApplication)).To(Succeed())

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should ONLY create ServiceEntry (NOT Secret or AzureAdApplication) when Entraid is enabled via clientRef, and SecurityConfig should have correct status", func() {
			const externalClientName = "external-azure-ad-application"

			By("Pre-creating a AzureAdApplication to be referenced via clientRef")
			externalClient := &naisiov1.AzureAdApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      externalClientName,
					Namespace: namespaceName,
				},
				Spec: naisiov1.AzureAdApplicationSpec{
					SecretName: "external-azure-ad-secret",
				},
			}
			Expect(k8sClient.Create(ctx, externalClient)).To(Succeed())
			DeferCleanup(func() {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: externalClientName, Namespace: namespaceName}, externalClient); err == nil {
					Expect(k8sClient.Delete(ctx, externalClient)).To(Succeed())
				}
			})

			By("Updating SecurityConfig with EntraID clientRef configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				EntraID: &accesseratorv1alpha.EntraIDSpec{
					Enabled: true,
					ClientRef: &accesseratorv1alpha.ResourceRef{
						Name: externalClientName,
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with EntraID clientRef")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that no new AzureAdApplication was created by the controller")
			controllerCreatedClient := &naisiov1.AzureAdApplication{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName(),
					Namespace: namespaceName,
				}, controllerCreatedClient))
			}).Should(BeTrue())

			By("Verifying that no Secret was created")
			secret := &corev1.Secret{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).SecretFromRefName(),
					Namespace: namespaceName,
				}, secret))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhasePending while waiting for the referenced AzureAdApplication")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the referenced AzureAdApplication as ready")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: externalClientName, Namespace: namespaceName}, externalClient)).To(Succeed())
			externalClient.Status.SynchronizationState = utilities.AzureAdApplicationSynchronizationStateReady
			externalClient.Status.SynchronizationSecretName = "external-azure-ad-application-secret"
			Expect(k8sClient.Status().Update(ctx, externalClient)).To(Succeed())

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should ONLY create a ServiceEntry and Secret (NOT AzureAdApplication) when Entraid is enabled via secretRef, and SecurityConfig should have correct status", func() {
			const (
				sourceSecretName = "entra-id-credentials"
				clientIDKey      = "clientId"
				clientJWKKey     = "clientJwk"
			)

			By("Pre-creating a Kubernetes Secret containing Entra ID credentials")
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceSecretName,
					Namespace: namespaceName,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					clientIDKey:  []byte("test-client-id"),
					clientJWKKey: []byte(`{"kty":"RSA","use":"sig","alg":"RS256","kid":"test"}`),
				},
			}
			Expect(k8sClient.Create(ctx, sourceSecret)).To(Succeed())
			DeferCleanup(func() {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceSecretName, Namespace: namespaceName}, sourceSecret); err == nil {
					Expect(k8sClient.Delete(ctx, sourceSecret)).To(Succeed())
				}
			})

			By("Updating SecurityConfig with EntraID secretRef configuration")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec = accesseratorv1alpha.SecurityConfigSpec{
				EntraID: &accesseratorv1alpha.EntraIDSpec{
					Enabled: true,
					SecretRef: &accesseratorv1alpha.SecretRef{
						ClientID: accesseratorv1alpha.SecretKeySelector{
							Name: sourceSecretName,
							Key:  clientIDKey,
						},
						ClientJWK: accesseratorv1alpha.SecretKeySelector{
							Name: sourceSecretName,
							Key:  clientJWKKey,
						},
					},
				},
				ApplicationRef: skiperatorAppName,
			}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with Entra ID secretRef")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a ServiceEntry resource was created")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			serviceEntryKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).ServiceEntryName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, serviceEntryKey, serviceEntry)
			}).Should(Succeed())

			By("Verifying that an integration Secret was created")
			integrationSecret := &corev1.Secret{}
			integrationSecretKey := types.NamespacedName{
				Name:      utilities.NewEntraIdNamer(securityConfig).SecretFromRefName(),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, integrationSecretKey, integrationSecret)
			}).Should(Succeed())

			By("Verifying that no AzureAdApplication was created")
			azureAdApplication := &naisiov1.AzureAdApplication{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName(),
					Namespace: namespaceName,
				}, azureAdApplication))
			}).Should(BeTrue())

			By("Verifying that SecurityConfig is PhaseReady (secretRef does not require waiting)")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				s := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, s); err != nil {
					return "", err
				}
				return s.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))
		})

		It("should NOT create MaskinportenClient, ServiceEntry or Secret when Maskinporten is disabled", func() {
			By("Setting Maskinporten to explicitly disabled on the SecurityConfig")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec.Maskinporten = &accesseratorv1alpha.MaskinportenSpec{Enabled: false}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with Maskinporten disabled")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that no MaskinportenClient resource exists")
			maskinportenClient := &naisiov1.MaskinportenClient{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
					Namespace: namespaceName,
				}, maskinportenClient))
			}).Should(BeTrue())

			By("Verifying that no ServiceEntry resource exists")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).ServiceEntryName(),
					Namespace: namespaceName,
				}, serviceEntry))
			}).Should(BeTrue())

			By("Verifying that no maskinporten Secret resource exists")
			secret := &corev1.Secret{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewMaskinportenNamer(securityConfig).SecretFromRefName(),
					Namespace: namespaceName,
				}, secret))
			}).Should(BeTrue())
		})

		It("should NOT create AzureAdApplication, ServiceEntry or Secret when EntraID is disabled", func() {
			By("Setting EntraID to explicitly disabled on the SecurityConfig")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Spec.EntraID = &accesseratorv1alpha.EntraIDSpec{Enabled: false}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig with EntraID disabled")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that no AzureAdApplication resource exists")
			azureAdApplication := &naisiov1.AzureAdApplication{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).AzureAdApplicationName(),
					Namespace: namespaceName,
				}, azureAdApplication))
			}).Should(BeTrue())

			By("Verifying that no ServiceEntry resource exists")
			serviceEntry := &istionetworkingv1.ServiceEntry{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).ServiceEntryName(),
					Namespace: namespaceName,
				}, serviceEntry))
			}).Should(BeTrue())

			By("Verifying that no entraid Secret resource exists")
			secret := &corev1.Secret{}
			Consistently(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewEntraIdNamer(securityConfig).SecretFromRefName(),
					Namespace: namespaceName,
				}, secret))
			}).Should(BeTrue())
		})

		It("should NOT create a Jwker resource nor a NetworkPolicy resource when TokenX is disabled", func() {
			By("Disabling TokenX on the SecurityConfig")
			securityConfig := accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{Name: securityConfigName},
				Spec:       accesseratorv1alpha.SecurityConfigSpec{ApplicationRef: skiperatorAppName},
			}
			Expect(k8sClient.Get(ctx, typeNamespacedName, &securityConfig)).To(Succeed())

			securityConfig.Spec.Tokenx = nil
			Expect(k8sClient.Update(ctx, &securityConfig)).To(Succeed())

			By("Reconciling the SecurityConfig with TokenX disabled")

			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that no Jwker resource exists")
			jwker := &naisiov1.Jwker{}
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewTokenxNamer(securityConfig).JwkerName(),
					Namespace: namespaceName,
				}, jwker)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no NetworkPolicy resource exists")
			netpol := &v1.NetworkPolicy{}
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.NewTokenxNamer(securityConfig).EgressName(config.Get().TokenxName),
					Namespace: namespaceName,
				}, netpol)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying events were emitted")
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileStarted")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconciledSuccessfully")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileSuccess")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("StatusUpdateSuccess")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("StatusUpdateFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
		})

		It("should recreate owned resources when they are deleted", func() {
			By("Reconciling the SecurityConfig to create owned resources")

			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the owned NetworkPolicy resource")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{
				Name:      utilities.NewTokenxNamer(securityConfig).EgressName(config.Get().TokenxName),
				Namespace: namespaceName,
			}
			Expect(k8sClient.Get(ctx, netpolKey, netpol)).To(Succeed())
			Expect(k8sClient.Delete(ctx, netpol)).To(Succeed())

			// In a real cluster, the controller would automatically reconcile when it detects the Jwker deletion. However, in envtest we need to manually trigger the reconciliation to simulate this behavior.
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the NetworkPolicy resource is recreated")
			Eventually(func() error {
				return k8sClient.Get(ctx, netpolKey, netpol)
			}).Should(Succeed())

			By("Deleting the owned Jwker resource")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{
				Name:      utilities.NewTokenxNamer(securityConfig).JwkerName(),
				Namespace: namespaceName,
			}
			Expect(k8sClient.Get(ctx, jwkerKey, jwker)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jwker)).To(Succeed())

			// In a real cluster, the controller would automatically reconcile when it detects the Jwker deletion. However, in envtest we need to manually trigger the reconciliation to simulate this behavior.
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the Jwker resource is recreated")
			Eventually(func() error {
				return k8sClient.Get(ctx, jwkerKey, jwker)
			}).Should(Succeed())
		})

		It("adds the standard labels to the SecurityConfig itself when they are missing", func() {
			By("Verifying the SecurityConfig starts without the standard labels")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			Expect(sc.Labels).NotTo(HaveKey(labels.ManagedByLabelKey))
			Expect(sc.Labels).NotTo(HaveKey(labels.ControllerLabelKey))

			By("Reconciling the SecurityConfig")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the standard labels were added to the SecurityConfig")
			Eventually(func(g Gomega) {
				updated := &accesseratorv1alpha.SecurityConfig{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
				g.Expect(updated.Labels).To(Equal(labels.SecurityConfigStandardLabels()))
			}).Should(Succeed())
		})

		It("preserves pre-existing labels on the SecurityConfig when adding the standard labels", func() {
			By("Adding a custom label to the SecurityConfig")
			sc := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
			sc.Labels = map[string]string{"custom.example.com/team": "platform"}
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			By("Reconciling the SecurityConfig")
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying both the custom label and the standard labels are present")
			Eventually(func(g Gomega) {
				updated := &accesseratorv1alpha.SecurityConfig{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
				g.Expect(updated.Labels).To(SatisfyAll(
					HaveKeyWithValue("custom.example.com/team", "platform"),
					HaveKeyWithValue(labels.ManagedByLabelKey, labels.ManagedByLabelValue),
					HaveKeyWithValue(labels.ControllerLabelKey, labels.SecurityConfigControllerLabelValue),
				))
			}).Should(Succeed())
		})

		It("patches the SecurityConfig labels exactly once and not again on a subsequent reconcile", func() {
			By("Building a reconciler whose client counts SecurityConfig patches")
			spyClient := &patchCountingClient{Client: gvkInjectingClient{k8sClient}}
			fakeRecorder := events.NewFakeRecorder(100)
			controllerReconciler := &controller.SecurityConfigReconciler{
				Client:   spyClient,
				Scheme:   spyClient.Scheme(),
				Recorder: fakeRecorder,
			}

			By("Reconciling once to add the standard labels")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
				g.Expect(sc.Labels).To(Equal(labels.SecurityConfigStandardLabels()))
			}).Should(Succeed())

			By("Verifying the first reconcile patched the SecurityConfig exactly once")
			Expect(spyClient.securityConfigPatches).To(Equal(1))

			By("Reconciling again now that the labels already match")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the second reconcile issued no further SecurityConfig patch")
			Expect(spyClient.securityConfigPatches).To(Equal(1))
		})
	})
})

var _ = Describe("SecurityConfigController Validation", func() {
	Context("When reconciling a resource", func() {
		const (
			securityConfigName = "test-resource"
			namespaceName      = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      securityConfigName,
			Namespace: namespaceName,
		}

		AfterEach(func() {
			resource := &accesseratorv1alpha.SecurityConfig{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SecurityConfig")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})

var _ = Describe("SecurityConfig status conditions and phase", func() {
	const (
		securityConfigName = "conditions-test-resource"
		skiperatorAppName  = "conditions-test-app"
		namespaceName      = "default"
	)

	typeNamespacedName := types.NamespacedName{
		Name:      securityConfigName,
		Namespace: namespaceName,
	}
	jwkerNamespacedName := types.NamespacedName{
		Name:      utilities.TokenxNamer{ApplicationRef: skiperatorAppName}.JwkerName(),
		Namespace: namespaceName,
	}
	netpolNamespacedName := types.NamespacedName{
		Name:      utilities.TokenxNamer{SecurityConfigName: securityConfigName}.EgressName(config.Get().TokenxName),
		Namespace: namespaceName,
	}

	var fakeRecorder *events.FakeRecorder
	var controllerReconciler *controller.SecurityConfigReconciler

	BeforeEach(func() {
		appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: namespaceName}
		existing := &v1alpha1.Application{}
		err := k8sClient.Get(ctx, appKey, existing)
		if err != nil && errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: namespaceName,
				},
				Spec: v1alpha1.ApplicationSpec{
					AccessPolicy: &podtypes.AccessPolicy{},
				},
			})).To(Succeed())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}

		fakeRecorder = events.NewFakeRecorder(100)
		controllerReconciler = getSecurityConfigReconciler(fakeRecorder)
	})

	AfterEach(func() {
		sc := &accesseratorv1alpha.SecurityConfig{}
		if err := k8sClient.Get(ctx, typeNamespacedName, sc); err == nil {
			Expect(k8sClient.Delete(ctx, sc)).To(Succeed())
		}
		jwker := &naisiov1.Jwker{}
		if err := k8sClient.Get(ctx, jwkerNamespacedName, jwker); err == nil {
			Expect(k8sClient.Delete(ctx, jwker)).To(Succeed())
		}
		netpol := &v1.NetworkPolicy{}
		if err := k8sClient.Get(ctx, netpolNamespacedName, netpol); err == nil {
			Expect(k8sClient.Delete(ctx, netpol)).To(Succeed())
		}
		app := &v1alpha1.Application{}
		appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: namespaceName}
		if err := k8sClient.Get(ctx, appKey, app); err == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
	})

	// hasConditionWithPrefix returns true if the SecurityConfig has at least
	// one condition whose Type begins with the given prefix.
	hasConditionWithPrefix := func(sc *accesseratorv1alpha.SecurityConfig, prefix string) bool {
		for _, c := range sc.Status.Conditions {
			if strings.HasPrefix(c.Type, prefix) {
				return true
			}
		}
		return false
	}

	getSecurityConfig := func() *accesseratorv1alpha.SecurityConfig {
		sc := &accesseratorv1alpha.SecurityConfig{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, sc)).To(Succeed())
		return sc
	}

	Describe("expected conditions based on spec", func() {
		It("includes Jwker and NetworkPolicy conditions when only TokenX is enabled", func() {
			Expect(k8sClient.Create(ctx, &accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      securityConfigName,
					Namespace: namespaceName,
				},
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
					Tokenx:         &accesseratorv1alpha.TokenXSpec{Enabled: true},
				},
			})).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				sc := getSecurityConfig()
				return hasConditionWithPrefix(sc, "Jwker-") && hasConditionWithPrefix(sc, "NetworkPolicy-")
			}).Should(BeTrue())

			sc := getSecurityConfig()
			Expect(hasConditionWithPrefix(sc, "MaskinportenClient-")).To(BeFalse())
			Expect(hasConditionWithPrefix(sc, "AzureAdApplication-")).To(BeFalse())
			Expect(hasConditionWithPrefix(sc, "ConfigMap-")).To(BeFalse())
		})

		It("includes no resource conditions when no security feature is enabled", func() {
			Expect(k8sClient.Create(ctx, &accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      securityConfigName,
					Namespace: namespaceName,
				},
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
					// no Tokenx, Maskinporten, EntraId, or Opa
				},
			})).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			sc := getSecurityConfig()
			Expect(hasConditionWithPrefix(sc, "Jwker-")).To(BeFalse())
			Expect(hasConditionWithPrefix(sc, "NetworkPolicy-")).To(BeFalse())
			Expect(hasConditionWithPrefix(sc, "MaskinportenClient-")).To(BeFalse())
			Expect(hasConditionWithPrefix(sc, "AzureAdApplication-")).To(BeFalse())
		})
	})

	Describe("phase when a same-named foreign-owned resource already exists", func() {
		// preCreateForeignJwker creates a Jwker at the same name the controller
		// would generate for the test SecurityConfig, with no OwnerReference
		// pointing at the SecurityConfig.
		preCreateForeignJwker := func() {
			Expect(k8sClient.Create(ctx, &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jwkerNamespacedName.Name,
					Namespace: jwkerNamespacedName.Namespace,
				},
				Spec: naisiov1.JwkerSpec{
					AccessPolicy: &naisiov1.AccessPolicy{},
					SecretName:   "foreign-secret",
				},
			})).To(Succeed())
		}

		It("sets Phase=Failed when reconciling a SecurityConfig that would have created the conflicting Jwker", func() {
			preCreateForeignJwker()

			Expect(k8sClient.Create(ctx, &accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      securityConfigName,
					Namespace: namespaceName,
				},
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
					Tokenx:         &accesseratorv1alpha.TokenXSpec{Enabled: true},
				},
			})).To(Succeed())

			// Reconcile is expected to return an error because of the ownership conflict.
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})

			Eventually(func() accesseratorv1alpha.Phase {
				return getSecurityConfig().Status.Phase
			}).Should(Equal(accesseratorv1alpha.PhaseFailed))

			Eventually(func() bool {
				for _, c := range getSecurityConfig().Status.Conditions {
					if c.Message == fmt.Sprintf(
						"Failed to reconcile Jwker %s/%s: cannot update %s/%s as it is not owned by SecurityConfig",
						namespaceName, jwkerNamespacedName.Name, namespaceName, jwkerNamespacedName.Name,
					) {
						return true
					}
				}
				return false
			}).Should(BeTrue())
		})

		It("transitions to Phase=Ready when the spec that triggered the conflict is removed", func() {
			preCreateForeignJwker()

			Expect(k8sClient.Create(ctx, &accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      securityConfigName,
					Namespace: namespaceName,
				},
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
					Tokenx:         &accesseratorv1alpha.TokenXSpec{Enabled: true},
				},
			})).To(Succeed())

			// First reconcile — Phase should land on Failed because of the conflict.
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Eventually(func() accesseratorv1alpha.Phase {
				return getSecurityConfig().Status.Phase
			}).Should(Equal(accesseratorv1alpha.PhaseFailed))

			Eventually(func() bool {
				for _, c := range getSecurityConfig().Status.Conditions {
					if c.Message == fmt.Sprintf(
						"Failed to reconcile Jwker %s/%s: cannot update %s/%s as it is not owned by SecurityConfig",
						namespaceName, jwkerNamespacedName.Name, namespaceName, jwkerNamespacedName.Name,
					) {
						return true
					}
				}
				return false
			}).Should(BeTrue())

			By("Removing Tokenx from the SecurityConfig spec")
			sc := getSecurityConfig()
			sc.Spec.Tokenx = nil
			Expect(k8sClient.Update(ctx, sc)).To(Succeed())

			// Reconcile again — without the Tokenx feature requested, the
			// foreign Jwker is no longer a conflict and reconciliation succeeds.
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() accesseratorv1alpha.Phase {
				return getSecurityConfig().Status.Phase
			}).Should(Equal(accesseratorv1alpha.PhaseReady))

			foreignJwker := &naisiov1.Jwker{}
			Expect(k8sClient.Get(ctx, jwkerNamespacedName, foreignJwker)).To(Succeed())
			Expect(foreignJwker.Spec.AccessPolicy.Inbound).To(BeNil())
			Expect(foreignJwker.Spec.AccessPolicy.Outbound).To(BeNil())
			Expect(foreignJwker.Spec.SecretName).To(Equal("foreign-secret"))
		})
	})
})

func getSecurityConfigReconciler(
	eventRecorder events.EventRecorder,
) *controller.SecurityConfigReconciler {
	return &controller.SecurityConfigReconciler{
		Client:   gvkInjectingClient{k8sClient},
		Scheme:   gvkInjectingClient{k8sClient}.Scheme(),
		Recorder: eventRecorder,
	}
}

// We need a wrapper of client.Client in order to override Get() to inject GroupVersionKind of SecurityConfig.
// This is needed as envtest does not populate TypeMeta on Get().
type gvkInjectingClient struct {
	client.Client
}

func (c gvkInjectingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if _, ok := obj.(*accesseratorv1alpha.SecurityConfig); ok {
		obj.GetObjectKind().SetGroupVersionKind(accesseratorv1alpha.GroupVersion.WithKind("SecurityConfig"))
	}
	return nil
}

// patchCountingClient wraps a client.Client and counts how many times the SecurityConfig itself is
// patched, so tests can assert the controller does not redundantly patch its own labels.
// Status subresource writes go through Status().Update and are intentionally not counted here.
type patchCountingClient struct {
	client.Client
	securityConfigPatches int
}

func (c *patchCountingClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	if _, ok := obj.(*accesseratorv1alpha.SecurityConfig); ok {
		c.securityConfigPatches++
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
