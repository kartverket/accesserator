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
			ConfigMapName: utilities.GetOpaConfigMapName(scope.SecurityConfig.Name),
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
	case len(scope.Descendants) != reconciliation.CountReconciledResources(controllerResources):
		return utilities.Ptr(StatePending), nil
	case len(scope.GetErrors()) > 0:
		return utilities.Ptr(StateFailed), nil
	}

	waitingForJwker := false
	waitingForMaskinportenClient := false

	if scope.TokenXConfig.Enabled {
		jwkerObjectKey := client.ObjectKey{
			Namespace: scope.SecurityConfig.Namespace,
			Name:      utilities.GetJwkerName(string(scope.SecurityConfig.Spec.ApplicationRef)),
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
		// If MaksinportenConfigType is secretRef, the integration secret is utilities.GetMaskinportenSecretFromSecretRefName(<SecurityConfig.Name>),
		// otherwise we need to fetch if from the MaskinportenClient status
		if scope.MaskinportenConfig.Type == state.SecretRef {
			scope.SecurityConfig.Status.MaskinportenSectretName = utilities.GetMaskinportenSecretFromSecretRefName(scope.SecurityConfig.Name)
		} else {
			var maskinportenClientName string
			switch scope.MaskinportenConfig.Type {
			case state.InlineClient, state.None:
				maskinportenClientName = utilities.GetMaskinportenClientName(string(scope.SecurityConfig.Spec.ApplicationRef))
			case state.ClientRef:
				maskinportenClientName = string(scope.SecurityConfig.Spec.Maskinporten.ClientRef.Name)
			default:
				return nil, fmt.Errorf("encountered invalid MaskinportenConfigType %d", scope.MaskinportenConfig.Type)
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
			scope.SecurityConfig.Status.MaskinportenSectretName = maskinportenClient.Status.SynchronizationSecretName
		}
	}
	switch {
	case waitingForJwker:
		return utilities.Ptr(StateWaitingForJwker), nil
	case waitingForMaskinportenClient:
		return utilities.Ptr(StateWaitingForMaskinportenClient), nil
	default:
		return utilities.Ptr(StateReady), nil
	}
}

func determinePhase(reconciliationState ReconciliationState) v1alpha.Phase {
	switch reconciliationState {
	case StateInvalid:
		return v1alpha.PhaseInvalid
	case StatePending, StateWaitingForJwker, StateWaitingForMaskinportenClient:
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
	case StateInvalid, StatePending, StateWaitingForJwker, StateWaitingForMaskinportenClient, StateFailed:
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
	case StateFailed:
		return "SecurityConfig failed."
	case StateReady:
		return "SecurityConfig ready."
	}
	panic("could not determine status message")
}
