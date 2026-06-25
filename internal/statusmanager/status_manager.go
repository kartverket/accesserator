package statusmanager

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/utilities"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconciliationState int

// NB: used in switch cases, ensure they are exhaustive.
const (
	StateInvalid ReconciliationState = iota
	StatePending
	StateWaitingForJwker
	StateWaitingForMaskinportenClient
	StateWaitingForAzureAdApplication
	StateFailed
	StateReady
)

// UpdateSecurityConfigStatus builds the new status, compares with the original, and updates if changed.
func UpdateSecurityConfigStatus(
	ctx context.Context,
	k8sClient client.Client,
	recorder events.EventRecorder,
	scope *state.Scope,
	originalSecurityConfig *v1alpha.SecurityConfig,
	controllerResources []reconciliation.ControllerResource,
) {
	sc := &scope.SecurityConfig
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating SecurityConfig status for %s/%s", sc.Namespace, sc.Name))
	recorder.Eventf(
		sc,
		nil,
		"Normal",
		"StatusUpdateStarted",
		"Reconcile",
		"Status update of SecurityConfig started.")

	reconciliationState, err := DetermineReconciliationState(ctx, k8sClient, scope, controllerResources)
	if err != nil {
		rLog.Error(err, "Failed to determine reconciliation state")
		recorder.Eventf(
			sc,
			nil,
			"Warning",
			"StatusUpdateFailed",
			"Reconcile",
			fmt.Sprintf("Failed to determine reconciliation state: %v", err),
		)
		return
	}

	sc.Status.ObservedGeneration = sc.GetGeneration()
	sc.Status.Phase = determinePhase(*reconciliationState)
	sc.Status.Ready = determineReadiness(*reconciliationState)
	sc.Status.Message = statusMessage(*reconciliationState, scope.ValidationErrorMessage)
	sc.Status.Conditions = BuildConditions(
		sc,
		*reconciliationState,
		scope.ValidationErrorMessage,
		scope.Descendants,
		controllerResources,
		originalSecurityConfig.Status.Conditions,
	)

	if scope.OpaConfig.Enabled {
		bundleNames := slices.Collect(maps.Keys(scope.OpaConfig.BundleBinaryData))
		slices.Sort(bundleNames)
		sc.Status.OpaBundleSource = &v1alpha.OpaBundleSource{
			ConfigMapName: utilities.NewOpaNamer(scope.SecurityConfig).ConfigMapName(),
			BundleNames:   bundleNames,
		}
	}

	if !equality.Semantic.DeepEqual(originalSecurityConfig.Status, sc.Status) {
		rLog.Debug(fmt.Sprintf("Updating SecurityConfig status with name %s/%s", sc.Namespace, sc.Name))
		if err := UpdateStatus(ctx, k8sClient, *sc); err != nil {
			rLog.Error(
				err,
				fmt.Sprintf("Failed to update SecurityConfig status with name %s/%s", sc.Namespace, sc.Name),
			)
			recorder.Eventf(sc, nil, "Warning", "StatusUpdateFailed", "Reconcile", "Status update of SecurityConfig failed.")
		} else {
			recorder.Eventf(sc, nil, "Normal", "StatusUpdateSuccess", "Reconcile", "Status update of SecurityConfig updated successfully.")
		}
	}
}

func DetermineReconciliationState(
	ctx context.Context,
	k8sClient client.Client,
	scope *state.Scope,
	controllerResources []reconciliation.ControllerResource,
) (*ReconciliationState, error) {
	switch {
	case scope.InvalidConfig:
		return utilities.Ptr(StateInvalid), nil
	case len(scope.GetErrors()) > 0:
		return utilities.Ptr(StateFailed), nil
	case len(scope.Descendants) != reconciliation.CountNonNilResources(controllerResources):
		return utilities.Ptr(StatePending), nil
	}

	waitingForJwker := false
	waitingForMaskinportenClient := false
	waitingForAzureAdApplication := false

	if scope.TokenXConfig.Enabled {
		jwkerObjectKey := client.ObjectKey{
			Namespace: scope.SecurityConfig.Namespace,
			Name:      utilities.NewTokenxNamer(scope.SecurityConfig).JwkerName(),
		}
		jwkerResource, getJwkerErr := utilities.GetJwker(ctx, k8sClient, jwkerObjectKey)
		if getJwkerErr != nil {
			return nil, fmt.Errorf("failed to get Jwker resource %s/%s: %w",
				jwkerObjectKey.Namespace,
				jwkerObjectKey.Name,
				getJwkerErr,
			)
		}
		if jwkerResource.Status.SynchronizationState != utilities.JwkerSynchronizationStateReady {
			waitingForJwker = true
		}
		scope.SecurityConfig.Status.JwkerSecretName = jwkerResource.Status.SynchronizationSecretName
	}

	if scope.MaskinportenConfig.Enabled {
		// If MaskinportenConfigType is secretRef, the integration secret is derived from the applicationRef.
		// Otherwise we need to fetch it from the MaskinportenClient status.
		if scope.MaskinportenConfig.Type == state.SecretRef {
			scope.SecurityConfig.Status.MaskinportenSecretName = utilities.NewMaskinportenNamer(scope.SecurityConfig).SecretFromRefName()
		} else {
			var maskinportenClientName string
			switch scope.MaskinportenConfig.Type {
			case state.InlineClient, state.None:
				maskinportenClientName = utilities.NewMaskinportenNamer(scope.SecurityConfig).MaskinportenClientName()
			case state.ClientRef:
				maskinportenClientName = string(scope.SecurityConfig.Spec.Maskinporten.ClientRef.Name)
			default:
				return nil, fmt.Errorf("encountered invalid ConfigType %d", scope.MaskinportenConfig.Type)
			}

			maskinportenClientObjectKey := client.ObjectKey{
				Namespace: scope.SecurityConfig.Namespace,
				Name:      maskinportenClientName,
			}
			maskinportenClient, getMaksinportenClientErr := utilities.GetMaskinportenClient(ctx, k8sClient, maskinportenClientObjectKey)
			if getMaksinportenClientErr != nil {
				return nil, fmt.Errorf("failed to get MaskinportenClient resource %s/%s: %w",
					maskinportenClientObjectKey.Namespace,
					maskinportenClientObjectKey.Name,
					getMaksinportenClientErr,
				)
			}
			if maskinportenClient.Status.SynchronizationState != utilities.MaskinportenClientSynchronizationStateReady {
				waitingForMaskinportenClient = true
			}
			scope.SecurityConfig.Status.MaskinportenSecretName = maskinportenClient.Status.SynchronizationSecretName
		}
	}

	if scope.EntraIdConfig.Enabled {
		if scope.EntraIdConfig.Type == state.SecretRef {
			scope.SecurityConfig.Status.EntraIdSecretName = utilities.NewEntraIdNamer(scope.SecurityConfig).SecretFromRefName()
		} else {
			var azureAdApplicationName string
			switch scope.EntraIdConfig.Type {
			case state.InlineClient, state.None:
				azureAdApplicationName = utilities.NewEntraIdNamer(scope.SecurityConfig).AzureAdApplicationName()
			case state.ClientRef:
				azureAdApplicationName = string(scope.SecurityConfig.Spec.EntraID.ClientRef.Name)
			default:
				return nil, fmt.Errorf("encountered invalid ConfigType %d", scope.EntraIdConfig.Type)
			}

			azureAdApplicationObjectKey := client.ObjectKey{
				Namespace: scope.SecurityConfig.Namespace,
				Name:      azureAdApplicationName,
			}
			azureAdApplication, getAzureAdApplicationErr := utilities.GetAzureAdApplication(ctx, k8sClient, azureAdApplicationObjectKey)
			if getAzureAdApplicationErr != nil {
				return nil, fmt.Errorf("failed to get AzureAdApplication resource %s/%s: %w",
					azureAdApplicationObjectKey.Namespace,
					azureAdApplicationObjectKey.Name,
					getAzureAdApplicationErr,
				)
			}
			if azureAdApplication.Status.SynchronizationState != utilities.AzureAdApplicationSynchronizationStateReady {
				waitingForAzureAdApplication = true
			}
			scope.SecurityConfig.Status.EntraIdSecretName = azureAdApplication.Status.SynchronizationSecretName
		}
	}

	if scope.IdPortenConfig.Enabled {
		scope.SecurityConfig.Status.IdportenAudience = scope.IdPortenConfig.Audience
	}

	switch {
	case waitingForJwker:
		return utilities.Ptr(StateWaitingForJwker), nil
	case waitingForMaskinportenClient:
		return utilities.Ptr(StateWaitingForMaskinportenClient), nil
	case waitingForAzureAdApplication:
		return utilities.Ptr(StateWaitingForAzureAdApplication), nil
	default:
		return utilities.Ptr(StateReady), nil
	}
}

func determinePhase(reconciliationState ReconciliationState) v1alpha.Phase {
	switch reconciliationState {
	case StateInvalid:
		return v1alpha.PhaseInvalid
	case StatePending, StateWaitingForJwker, StateWaitingForMaskinportenClient, StateWaitingForAzureAdApplication:
		return v1alpha.PhasePending
	case StateFailed:
		return v1alpha.PhaseFailed
	case StateReady:
		return v1alpha.PhaseReady
	}
	panic("could not determine phase")
}

func determineReadiness(reconciliationState ReconciliationState) bool {
	switch reconciliationState {
	case StateInvalid, StatePending, StateWaitingForJwker, StateWaitingForMaskinportenClient, StateWaitingForAzureAdApplication, StateFailed:
		return false
	case StateReady:
		return true
	}
	panic("could not determine readiness")
}

func statusMessage(reconciliationState ReconciliationState, validationErrorMessage *string) string {
	switch reconciliationState {
	case StateInvalid:
		return *validationErrorMessage
	case StatePending:
		return "SecurityConfig pending due to missing Descendants."
	case StateWaitingForJwker:
		return "SecurityConfig pending, waiting for Jwker to be ready."
	case StateWaitingForMaskinportenClient:
		return "SecurityConfig pending, waiting for MaskinportenClient to be ready."
	case StateWaitingForAzureAdApplication:
		return "SecurityConfig pending, waiting for AzureAdApplication to be ready."
	case StateFailed:
		return "SecurityConfig failed."
	case StateReady:
		return "SecurityConfig ready."
	}
	panic("could not determine status message")
}
