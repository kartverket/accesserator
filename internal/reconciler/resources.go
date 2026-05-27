package reconciler

import (
	"reflect"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/entraid/azureadapplication"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/entraid/azureadsecret"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/entraid/azureadserviceentry"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportenclient"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportensecret"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportenserviceentry"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/opa"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/egress"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/jwker"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerResources creates all resource adapters for the given SecurityConfig
func ControllerResources(scope *state.Scope) []reconciliation.ControllerResource {
	if scope.InvalidConfig {
		return nil
	}

	return []reconciliation.ControllerResource{
		jwkerControllerResource(scope),
		tokenxEgressControllerResource(scope),
		maskinportenClientControllerResource(scope),
		maskinportenSecretControllerResource(scope),
		maskinportenServiceEntryControllerResource(scope),
		azureAdApplicationControllerResource(scope),
		azureAdSecretControllerResource(scope),
		azureAdServiceEntryControllerResource(scope),
		opaConfigMapControllerResource(scope),
	}
}

/*
jwkerControllerResource reconciles a Jwker resource which creates a client registration in the token exchange server and
a corresponding secret which may be used to authenticate the Texas sidecar client.
*/
func jwkerControllerResource(scope *state.Scope) ControllerResourceAdapter[*naisiov1.Jwker] {
	jwkerObjectMeta := metav1.ObjectMeta{
		Name:      utilities.JwkerNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.Name(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := jwker.GetDesired(jwkerObjectMeta, scope.TokenXConfig)

	return ControllerResourceAdapter[*naisiov1.Jwker]{
		reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
			Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
				ResourceKind:    "Jwker",
				ResourceName:    jwkerObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
					return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
				},
				UpdateFields: func(current, desired *naisiov1.Jwker) {
					current.Spec = desired.Spec
				},
			},
		},
	}
}

/*
tokenxEgressControllerResource reconciles a NetworkPolicy resource which allows egress traffic to the token exchange
server, if token exchange is enabled.
*/
func tokenxEgressControllerResource(scope *state.Scope) ControllerResourceAdapter[*networkv1.NetworkPolicy] {
	tokenxEgressObjectMeta := metav1.ObjectMeta{
		Name:      utilities.JwkerNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.EgressName(config.Get().TokenxName),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := egress.GetDesired(tokenxEgressObjectMeta, scope.TokenXConfig)

	return ControllerResourceAdapter[*networkv1.NetworkPolicy]{
		reconciliation.ReconcilerAdapter[*networkv1.NetworkPolicy]{
			Func: reconciliation.ResourceReconciler[*networkv1.NetworkPolicy]{
				ResourceKind:    "NetworkPolicy",
				ResourceName:    tokenxEgressObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate: func(current, desired *networkv1.NetworkPolicy) bool {
					return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
				},
				UpdateFields: func(current, desired *networkv1.NetworkPolicy) {
					current.Spec = desired.Spec
				},
			},
		},
	}
}

/*
maskinportenClientControllerResource reconciles a MaskinportenClient resources, which registers a client through
Digdir's API with corresponding secret that may be used to fetch Maskinporten tokens.
*/
func maskinportenClientControllerResource(scope *state.Scope) ControllerResourceAdapter[*naisiov1.MaskinportenClient] {
	maskinportenClientObjectMeta := metav1.ObjectMeta{
		Name:      utilities.MaskinportenNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.Name(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := maskinportenclient.GetDesired(maskinportenClientObjectMeta, scope.MaskinportenConfig)

	return ControllerResourceAdapter[*naisiov1.MaskinportenClient]{
		reconciliation.ReconcilerAdapter[*naisiov1.MaskinportenClient]{
			Func: reconciliation.ResourceReconciler[*naisiov1.MaskinportenClient]{
				ResourceKind:    "MaskinportenClient",
				ResourceName:    maskinportenClientObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate: func(current, desired *naisiov1.MaskinportenClient) bool {
					return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
				},
				UpdateFields: func(current, desired *naisiov1.MaskinportenClient) {
					current.Spec = desired.Spec
				},
			},
		},
	}
}

/*
maskinportenSecretControllerResource reconciles a Secret resource based on an existing Maskinporten client registration.
*/
func maskinportenSecretControllerResource(scope *state.Scope) ControllerResourceAdapter[*corev1.Secret] {
	maskinportenSecretObjectMeta := metav1.ObjectMeta{
		Name:      utilities.MaskinportenNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.SecretFromRefName(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := maskinportensecret.GetDesired(maskinportenSecretObjectMeta, scope.MaskinportenConfig)

	return ControllerResourceAdapter[*corev1.Secret]{
		reconciliation.ReconcilerAdapter[*corev1.Secret]{
			Func: reconciliation.ResourceReconciler[*corev1.Secret]{
				ResourceKind:    "Secret",
				ResourceName:    maskinportenSecretObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    SecretShouldUpdateFunc,
				UpdateFields:    secretUpdateFieldsFunc,
			},
		},
	}
}

/*
maskinportenServiceEntryControllerResource reconciles a ServiceEntry resource which allows access to the Maskinporten
API.
*/
func maskinportenServiceEntryControllerResource(scope *state.Scope) ControllerResourceAdapter[*istionetworkingv1.ServiceEntry] {
	maskinportenServiceEntryObjectMeta := metav1.ObjectMeta{
		Name:      utilities.MaskinportenNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.ServiceEntryName(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := maskinportenserviceentry.GetDesired(maskinportenServiceEntryObjectMeta, scope.MaskinportenConfig)

	return ControllerResourceAdapter[*istionetworkingv1.ServiceEntry]{
		reconciliation.ReconcilerAdapter[*istionetworkingv1.ServiceEntry]{
			Func: reconciliation.ResourceReconciler[*istionetworkingv1.ServiceEntry]{
				ResourceKind:    "ServiceEntry",
				ResourceName:    maskinportenServiceEntryObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    ServiceEntryShouldUpdateFunc,
				UpdateFields:    serviceEntryUpdateFieldsFunc,
			},
		},
	}
}

/*
azureAdApplicationControllerResource reconciles a AzureAdApplication resources, which registers a client through
Azure's API with corresponding secret that may be used to fetch Entra ID tokens.
*/
func azureAdApplicationControllerResource(scope *state.Scope) ControllerResourceAdapter[*naisiov1.AzureAdApplication] {
	azureAdApplicationObjectMeta := metav1.ObjectMeta{
		Name:      utilities.EntraIdNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.Name(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := azureadapplication.GetDesired(azureAdApplicationObjectMeta, scope.EntraIdConfig)

	return ControllerResourceAdapter[*naisiov1.AzureAdApplication]{
		reconciliation.ReconcilerAdapter[*naisiov1.AzureAdApplication]{
			Func: reconciliation.ResourceReconciler[*naisiov1.AzureAdApplication]{
				ResourceKind:    "AzureAdApplication",
				ResourceName:    azureAdApplicationObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate: func(current, desired *naisiov1.AzureAdApplication) bool {
					return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
				},
				UpdateFields: func(current, desired *naisiov1.AzureAdApplication) {
					current.Spec = desired.Spec
				},
			},
		},
	}
}

/*
azureAdSecretControllerResource reconciles a Secret resource based on an existing Entra ID client registration.
*/
func azureAdSecretControllerResource(scope *state.Scope) ControllerResourceAdapter[*corev1.Secret] {
	azureAdSecretObjectMeta := metav1.ObjectMeta{
		Name:      utilities.EntraIdNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.SecretFromRefName(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := azureadsecret.GetDesired(azureAdSecretObjectMeta, scope.EntraIdConfig)

	return ControllerResourceAdapter[*corev1.Secret]{
		reconciliation.ReconcilerAdapter[*corev1.Secret]{
			Func: reconciliation.ResourceReconciler[*corev1.Secret]{
				ResourceKind:    "Secret",
				ResourceName:    azureAdSecretObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    SecretShouldUpdateFunc,
				UpdateFields:    secretUpdateFieldsFunc,
			},
		},
	}
}

/*
azureAdServiceEntryControllerResource reconciles a ServiceEntry resource which allows access to the Entra ID API.
*/
func azureAdServiceEntryControllerResource(scope *state.Scope) ControllerResourceAdapter[*istionetworkingv1.ServiceEntry] {
	azureAdServiceEntryObjectMeta := metav1.ObjectMeta{
		Name:      utilities.EntraIdNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.ServiceEntryName(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := azureadserviceentry.GetDesired(azureAdServiceEntryObjectMeta, scope.EntraIdConfig)

	return ControllerResourceAdapter[*istionetworkingv1.ServiceEntry]{
		reconciliation.ReconcilerAdapter[*istionetworkingv1.ServiceEntry]{
			Func: reconciliation.ResourceReconciler[*istionetworkingv1.ServiceEntry]{
				ResourceKind:    "ServiceEntry",
				ResourceName:    azureAdServiceEntryObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    ServiceEntryShouldUpdateFunc,
				UpdateFields:    serviceEntryUpdateFieldsFunc,
			},
		},
	}
}

/*
opaConfigMapControllerResource reconciles a ConfigMap resource with all configured bundles as binary data.
*/
func opaConfigMapControllerResource(scope *state.Scope) ControllerResourceAdapter[*corev1.ConfigMap] {
	opaConfigMapObjectMeta := metav1.ObjectMeta{
		Name:      utilities.OpaNamer{Base: string(scope.SecurityConfig.Spec.ApplicationRef)}.ConfigMapName(),
		Namespace: scope.SecurityConfig.Namespace,
	}
	desiredResource := opa.GetDesired(opaConfigMapObjectMeta, scope.OpaConfig)

	return ControllerResourceAdapter[*corev1.ConfigMap]{
		reconciliation.ReconcilerAdapter[*corev1.ConfigMap]{
			Func: reconciliation.ResourceReconciler[*corev1.ConfigMap]{
				ResourceKind:    "ConfigMap",
				ResourceName:    opaConfigMapObjectMeta.Name,
				DesiredResource: utilities.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    ConfigMapShouldUpdateFunc,
				UpdateFields:    configMapUpdateFieldsFunc,
			},
		},
	}
}

func ConfigMapShouldUpdateFunc(current, desired *corev1.ConfigMap) bool {
	return !equality.Semantic.DeepEqual(current.BinaryData, desired.BinaryData) ||
		!equality.Semantic.DeepEqual(current.Data, desired.Data) ||
		!equality.Semantic.DeepEqual(current.Immutable, desired.Immutable)
}

func configMapUpdateFieldsFunc(current, desired *corev1.ConfigMap) {
	current.BinaryData = desired.BinaryData
	current.Data = desired.Data
	current.Immutable = desired.Immutable
}

func SecretShouldUpdateFunc(current, desired *corev1.Secret) bool {
	return !equality.Semantic.DeepEqual(current.StringData, desired.StringData) ||
		!equality.Semantic.DeepEqual(current.Data, desired.Data) ||
		!equality.Semantic.DeepEqual(current.Immutable, desired.Immutable)
}

func secretUpdateFieldsFunc(current, desired *corev1.Secret) {
	current.StringData = desired.StringData
	current.Data = desired.Data
	current.Immutable = desired.Immutable
	current.Type = desired.Type
}

func ServiceEntryShouldUpdateFunc(current, desired *istionetworkingv1.ServiceEntry) bool {
	return !reflect.DeepEqual(current.Spec.GetExportTo(), desired.Spec.GetExportTo()) ||
		!reflect.DeepEqual(current.Spec.GetHosts(), desired.Spec.GetHosts()) ||
		!reflect.DeepEqual(current.Spec.GetPorts(), desired.Spec.GetPorts()) ||
		!reflect.DeepEqual(current.Spec.GetResolution(), desired.Spec.GetResolution())
}

func serviceEntryUpdateFieldsFunc(current, desired *istionetworkingv1.ServiceEntry) {
	current.Spec.ExportTo = desired.Spec.ExportTo
	current.Spec.Hosts = desired.Spec.Hosts
	current.Spec.Ports = desired.Spec.Ports
	current.Spec.Resolution = desired.Spec.Resolution
}
