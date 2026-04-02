package controller

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/internal/statusmanager"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportenclient"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportensecret"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportenserviceentry"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/egress"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/jwker"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecurityConfigReconciler reconciles a SecurityConfig object
type SecurityConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecurityConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&accesseratorv1alpha.SecurityConfig{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&naisiov1.Jwker{}).
		Owns(&naisiov1.MaskinportenClient{}).
		Owns(&networkv1.NetworkPolicy{}).
		Owns(&corev1.Secret{}).
		Watches(&v1alpha1.Application{}, eventhandler.HandleSkiperatorApplicationEvent(r.Client)).
		Named("securityconfig").
		Complete(r)
}

// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=skiperator.kartverket.no,resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups=nais.io,resources=jwkers;maskinportenclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=serviceentries,verbs=get;list;watch;create;update;patch;delete

func (r *SecurityConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rlog := log.GetLogger(ctx)
	securityConfig := new(accesseratorv1alpha.SecurityConfig)
	rlog.Info("Reconciling SecurityConfig", "name", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, securityConfig); err != nil {
		if apierrors.IsNotFound(err) {
			rlog.Debug("SecurityConig with not found. Probably a delete.", "name", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		rlog.Error(err, "failed to get SecurityConfig", "name", req.NamespacedName)
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(
		securityConfig,
		nil,
		"Normal",
		"ReconcileStarted",
		"Reconcile",
		"SecurityConfig with name %s started.", req.String(),
	)

	rlog.Debug("SecurityConfig found", "name", req.NamespacedName)

	securityConfig.InitializeStatus()
	deepCopiedSecurityConfig := securityConfig.DeepCopy()

	if !securityConfig.DeletionTimestamp.IsZero() {
		rlog.Info("SecurityConfig is marked for deletion.", "name", req.NamespacedName)
		return reconcile.Result{}, nil
	}

	scope, err := resolver.ResolveSecurityConfig(ctx, r.Client, *securityConfig)
	if err != nil {
		rlog.Error(err, "failed to resolve SecurityConfig", "name", req.NamespacedName)
		securityConfig.Status.Phase = accesseratorv1alpha.PhaseFailed
		securityConfig.Status.Message = err.Error()
		updateStatusOnResolveFailedErr := statusmanager.UpdateStatus(ctx, r.Client, *securityConfig)
		if updateStatusOnResolveFailedErr != nil {
			return ctrl.Result{}, updateStatusOnResolveFailedErr
		}
		return reconcile.Result{}, err
	}

	jwkerObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetJwkerName(string(securityConfig.Spec.ApplicationRef)),
		Namespace: securityConfig.Namespace,
	}
	tokenxEgressObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetTokenxEgressName(securityConfig.Name, config.Get().TokenxName),
		Namespace: securityConfig.Namespace,
	}
	maskinportenClientObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetMaskinportenClientName(string(securityConfig.Spec.ApplicationRef)),
		Namespace: securityConfig.Namespace,
	}
	maskinportenSecretObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetMaskinportenSecretFromSecretRefName(securityConfig.Name),
		Namespace: securityConfig.Namespace,
	}
	maskinportenServiceEntryObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetMaskinportenServiceEntryName(securityConfig.Name),
		Namespace: securityConfig.Namespace,
	}

	controllerResources := []reconciliation.ControllerResource{
		ControllerResourceAdapter[*naisiov1.Jwker]{
			reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
				Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
					ResourceKind:    "Jwker",
					ResourceName:    jwkerObjectMeta.Name,
					DesiredResource: utilities.Ptr(jwker.GetDesired(jwkerObjectMeta, scope.TokenXConfig)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
						return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
					},
					UpdateFields: func(current, desired *naisiov1.Jwker) {
						current.Spec = desired.Spec
					},
				},
			},
		},
		ControllerResourceAdapter[*networkv1.NetworkPolicy]{
			reconciliation.ReconcilerAdapter[*networkv1.NetworkPolicy]{
				Func: reconciliation.ResourceReconciler[*networkv1.NetworkPolicy]{
					ResourceKind:    "NetworkPolicy",
					ResourceName:    tokenxEgressObjectMeta.Name,
					DesiredResource: utilities.Ptr(egress.GetDesired(tokenxEgressObjectMeta, scope.TokenXConfig)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *networkv1.NetworkPolicy) bool {
						return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
					},
					UpdateFields: func(current, desired *networkv1.NetworkPolicy) {
						current.Spec = desired.Spec
					},
				},
			},
		},
		ControllerResourceAdapter[*naisiov1.MaskinportenClient]{
			reconciliation.ReconcilerAdapter[*naisiov1.MaskinportenClient]{
				Func: reconciliation.ResourceReconciler[*naisiov1.MaskinportenClient]{
					ResourceKind:    "MaskinportenClient",
					ResourceName:    maskinportenSecretObjectMeta.Name,
					DesiredResource: utilities.Ptr(maskinportenclient.GetDesired(maskinportenClientObjectMeta, scope.MaskinportenConfig)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *naisiov1.MaskinportenClient) bool {
						return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
					},
					UpdateFields: func(current, desired *naisiov1.MaskinportenClient) {
						current.Spec = desired.Spec
					},
				},
			},
		},
		ControllerResourceAdapter[*corev1.Secret]{
			reconciliation.ReconcilerAdapter[*corev1.Secret]{
				Func: reconciliation.ResourceReconciler[*corev1.Secret]{
					ResourceKind:    "Secret",
					ResourceName:    maskinportenSecretObjectMeta.Name,
					DesiredResource: utilities.Ptr(maskinportensecret.GetDesired(maskinportenSecretObjectMeta, scope.MaskinportenConfig)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *corev1.Secret) bool {
						if len(current.Data) != len(desired.Data) {
							return true
						}
						for key, desiredVal := range desired.Data {
							if !bytes.Equal(current.Data[key], desiredVal) {
								return true
							}
						}
						return false
					},
					UpdateFields: func(current, desired *corev1.Secret) {
						current.Data = desired.Data
						current.Type = desired.Type
					},
				},
			},
		},
		ControllerResourceAdapter[*istionetworkingv1.ServiceEntry]{
			reconciliation.ReconcilerAdapter[*istionetworkingv1.ServiceEntry]{
				Func: reconciliation.ResourceReconciler[*istionetworkingv1.ServiceEntry]{
					ResourceKind:    "ServiceEntry",
					ResourceName:    maskinportenSecretObjectMeta.Name,
					DesiredResource: utilities.Ptr(maskinportenserviceentry.GetDesired(maskinportenServiceEntryObjectMeta, scope.MaskinportenConfig.Enabled)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *istionetworkingv1.ServiceEntry) bool {
						return !reflect.DeepEqual(current.Spec.GetExportTo(), desired.Spec.GetExportTo()) ||
							!reflect.DeepEqual(current.Spec.GetHosts(), desired.Spec.GetHosts()) ||
							!reflect.DeepEqual(current.Spec.GetPorts(), desired.Spec.GetPorts()) ||
							!reflect.DeepEqual(current.Spec.GetResolution(), desired.Spec.GetResolution())
					},
					UpdateFields: func(current, desired *istionetworkingv1.ServiceEntry) {
						current.Spec.ExportTo = desired.Spec.ExportTo
						current.Spec.Hosts = desired.Spec.Hosts
						current.Spec.Ports = desired.Spec.Ports
						current.Spec.Resolution = desired.Spec.Resolution
					},
				},
			},
		},
	}

	defer func() {
		statusmanager.UpdateSecurityConfigStatus(
			ctx,
			r.Client,
			r.Recorder,
			scope,
			deepCopiedSecurityConfig,
			controllerResources,
		)
	}()

	return r.doReconcile(ctx, controllerResources, scope)
}

func (r *SecurityConfigReconciler) doReconcile(
	ctx context.Context,
	controllerResources []reconciliation.ControllerResource,
	scope *state.Scope,
) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range controllerResources {
		reconcileResult, err := rf.Reconcile(ctx, r.Client, r.Scheme)
		if err != nil {
			r.Recorder.Eventf(
				&scope.SecurityConfig,
				nil,
				"Warning",
				fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()),
				"Reconcile",
				"%s with name %s failed during reconciliation.",
				rf.GetResourceKind(),
				rf.GetResourceName(),
			)
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(&scope.SecurityConfig, nil, "Normal", fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()), "Reconcile", "%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName())
		}
		if len(errs) > 0 {
			continue
		}
		result = utilities.LowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(&scope.SecurityConfig, nil, "Warning", "ReconcileFailed", "Reconcile", "SecurityConfig failed during reconciliation")
		return ctrl.Result{}, k8sErrors.NewAggregate(errs)
	}
	r.Recorder.Eventf(&scope.SecurityConfig, nil, "Normal", "ReconcileSuccess", "Reconcile", "SecurityConfig reconciled successfully")
	return result, nil
}
