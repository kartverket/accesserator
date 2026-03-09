package statusmanager

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildConditions builds all conditions for the SecurityConfig status.
func BuildConditions(
	securityConfig *v1alpha.SecurityConfig,
	reconciliationState ReconciliationState,
	validationErrorMessage *string,
	descendants []state.Descendant[client.Object],
	controllerResources []reconciliation.ControllerResource,
	existingConditions []metav1.Condition,
) []metav1.Condition {
	securityConfigCondition := BuildSecurityConfigCondition(
		securityConfig,
		reconciliationState,
		validationErrorMessage,
		existingConditions,
	)
	descendantConditions := BuildDescendantConditions(descendants, existingConditions)
	missingResourceConditions := BuildMissingResourceConditions(descendants, controllerResources, existingConditions)

	return slices.Concat([]metav1.Condition{securityConfigCondition}, descendantConditions, missingResourceConditions)
}

// BuildSecurityConfigCondition builds the SecurityConfig condition based on reconciliation state.
func BuildSecurityConfigCondition(
	securityConfig *v1alpha.SecurityConfig,
	reconciliationState ReconciliationState,
	validationErrorMessage *string,
	existingConditions []metav1.Condition,
) metav1.Condition {
	conditionType := state.GetID(strings.TrimPrefix(securityConfig.Kind, "*"), securityConfig.Name)

	condition := metav1.Condition{
		Type:               conditionType,
		LastTransitionTime: metav1.Now(),
	}

	switch reconciliationState {
	case StateInvalid:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "InvalidConfiguration"
		condition.Message = *validationErrorMessage

	case StatePending:
		condition.Status = metav1.ConditionUnknown
		condition.Reason = "ReconciliationPending"
		condition.Message = "Descendants of SecurityConfig are not yet reconciled."

	case StateWaitingForJwker:
		condition.Status = metav1.ConditionUnknown
		condition.Reason = "ReconciliationPending"
		condition.Message = "Jwker resource has not finished reconciliation."

	case StateFailed:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ReconciliationFailed"
		condition.Message = "Descendants of SecurityConfig failed during reconciliation."

	case StateReady:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "ReconciliationSuccess"
		condition.Message = "Descendants of SecurityConfig reconciled successfully."
	}

	// Preserve LastTransitionTime if the condition is unchanged
	for _, existing := range existingConditions {
		if isLogicallyEqualCondition(existing, condition) {
			condition.LastTransitionTime = existing.LastTransitionTime
			break
		}
	}

	return condition
}

// BuildDescendantConditions builds conditions for all descendants.
func BuildDescendantConditions(
	descendants []state.Descendant[client.Object],
	existingConditions []metav1.Condition,
) []metav1.Condition {
	conditions := make([]metav1.Condition, 0, len(descendants))

	for _, d := range descendants {
		condition := metav1.Condition{
			Type:               d.ID,
			LastTransitionTime: metav1.Now(),
		}

		switch {
		case d.ErrorMessage != nil:
			condition.Status = metav1.ConditionFalse
			condition.Reason = "Error"
			condition.Message = *d.ErrorMessage
		case d.SuccessMessage != nil:
			condition.Status = metav1.ConditionTrue
			condition.Reason = "Success"
			condition.Message = *d.SuccessMessage
		default:
			condition.Status = metav1.ConditionUnknown
			condition.Reason = "Unknown"
			condition.Message = "No status message set"
		}

		// Preserve LastTransitionTime if the condition is unchanged
		for _, existing := range existingConditions {
			if isLogicallyEqualCondition(existing, condition) {
				condition.LastTransitionTime = existing.LastTransitionTime
				break
			}
		}

		conditions = append(conditions, condition)
	}

	return conditions
}

// BuildMissingResourceConditions builds conditions for resources that were expected but not found.
func BuildMissingResourceConditions(
	descendants []state.Descendant[client.Object],
	controllerResources []reconciliation.ControllerResource,
	existingConditions []metav1.Condition,
) []metav1.Condition {
	var conditions []metav1.Condition

	// Map of existing descendant IDs
	descendantIDs := make(map[string]bool)
	for _, d := range descendants {
		descendantIDs[d.ID] = true
	}

	for _, controllerResource := range controllerResources {
		if !controllerResource.IsResourceNil() {
			expectedID := state.GetID(controllerResource.GetResourceKind(), controllerResource.GetResourceName())
			if !descendantIDs[expectedID] {
				condition := metav1.Condition{
					Type:   expectedID,
					Status: metav1.ConditionFalse,
					Reason: "NotFound",
					Message: fmt.Sprintf(
						"Expected resource %s of kind %s was not created",
						controllerResource.GetResourceName(),
						controllerResource.GetResourceKind(),
					),
					LastTransitionTime: metav1.Now(),
				}

				// Preserve LastTransitionTime if the condition is unchanged
				for _, existing := range existingConditions {
					if isLogicallyEqualCondition(existing, condition) {
						condition.LastTransitionTime = existing.LastTransitionTime
						break
					}
				}

				conditions = append(conditions, condition)
			}
		}
	}

	return conditions
}

func isLogicallyEqualCondition(existing metav1.Condition, cond metav1.Condition) bool {
	return existing.Type == cond.Type &&
		existing.Status == cond.Status &&
		existing.Reason == cond.Reason &&
		existing.Message == cond.Message
}
