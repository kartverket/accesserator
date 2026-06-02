package reconciliation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RequiresCreateAction ReconcileAction = iota
	RequiresUpdateAction
	RequiresDeleteAction
	RequiresNoAction
)

type ReconcileAction int

func (reconcileAction ReconcileAction) Action() string {
	switch reconcileAction {
	case RequiresCreateAction:
		return "creation"
	case RequiresUpdateAction:
		return "update"
	case RequiresDeleteAction:
		return "deletion"
	case RequiresNoAction:
		return "no action"
	}
	panic("Unknown reconcile action")
}

type ControllerResource interface {
	Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error)
	GetResourceKind() string
	GetResourceName() string
	IsResourceNil() bool
}

type ReconcilerAdapter[T client.Object] struct {
	Func ResourceReconciler[T]
}

type ResourceReconciler[T client.Object] struct {
	ResourceKind    string
	ResourceName    string
	DesiredResource *T
	Scope           *state.Scope
	ShouldUpdate    func(current T, desired T) bool
	UpdateFields    func(current T, desired T)
}

func CountNonNilResources(rfs []ControllerResource) int {
	count := 0
	for _, rf := range rfs {
		if !rf.IsResourceNil() {
			count++
		}
	}
	return count
}

func ReconcileControllerResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *state.Scope,
	resourceKind, resourceName string,
	desired *T,
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	resourceType := reflect.TypeOf((*T)(nil)).Elem()
	current, _ := reflect.New(resourceType.Elem()).Interface().(T)
	current.SetNamespace(scope.SecurityConfig.Namespace)
	current.SetName(resourceName)

	currentExists := true
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(current), current); err != nil {
		if apierrors.IsNotFound(err) {
			currentExists = false
		} else {
			errorReason := fmt.Sprintf(
				"Unable to get %s %s/%s.",
				resourceKind,
				current.GetNamespace(),
				current.GetName(),
			)
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}
	}

	currentIsOwnedBySecurityConfig := metav1.IsControlledBy(current, &scope.SecurityConfig)

	desiredIsNil := desired == nil || reflect.ValueOf(*desired).IsNil()

	rLog.Info(
		fmt.Sprintf("Determining reconcile action for %s %s/%s", resourceKind, current.GetNamespace(), current.GetName()),
	)
	reconcileAction, err := DetermineReconcileAction[T](
		current,
		desired,
		desiredIsNil,
		shouldUpdate,
		currentExists,
		currentIsOwnedBySecurityConfig,
	)

	if err != nil {
		errorReason := fmt.Sprintf(
			"Failed to reconcile %s %s/%s: %s",
			resourceKind,
			current.GetNamespace(),
			current.GetName(),
			err,
		)
		scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Info(
		fmt.Sprintf("%s %s/%s needs %s", resourceKind, current.GetNamespace(), current.GetName(), reconcileAction.Action()),
	)

	switch *reconcileAction {
	case RequiresDeleteAction:
		return ReconcileOnDelete[T](
			rLog,
			ctx,
			k8sClient,
			scope,
			current,
			currentIsOwnedBySecurityConfig,
		)
	case RequiresCreateAction:
		return ReconcileOnCreate[T](rLog, ctx, scheme, scope, k8sClient, *desired)
	case RequiresUpdateAction:
		return ReconcileOnUpdate[T](
			rLog,
			ctx,
			k8sClient,
			scope,
			*desired,
			current,
			updateFields,
			currentIsOwnedBySecurityConfig,
		)
	case RequiresNoAction:
		rLog.Debug(
			fmt.Sprintf(
				"No action needed for %s %s/%s.",
				resourceKind,
				current.GetNamespace(),
				current.GetName(),
			),
		)
		if !desiredIsNil {
			successMessage := fmt.Sprintf(
				"Successfully reconciled %s %s/%s.",
				resourceKind,
				current.GetNamespace(),
				resourceName,
			)
			rLog.Info(successMessage)
			scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, fmt.Errorf(
		"encountered unknown reconcile action when determining reconcile action for %s %s/%s",
		resourceKind,
		current.GetNamespace(),
		current.GetName(),
	)
}

func ReconcileOnUpdate[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	scope *state.Scope,
	desired T,
	current T,
	updateFields func(current, desired T),
	currentIsOwnedBySecurityConfig bool,
) (ctrl.Result, error) {
	kind := reflect.TypeOf(desired).Elem().Name()

	if !currentIsOwnedBySecurityConfig {
		err := fmt.Errorf(
			"%s %s/%s is not owned by SecurityConfig %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		)
		rLog.Error(
			err,
			"Cannot update resource that is not owned by SecurityConfig.",
		)
		errMsg := fmt.Sprintf(
			"Cannot update %s %s/%s as it is not owned by SecurityConfig %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		)
		scope.ReplaceDescendant(current, &errMsg, nil, kind, current.GetName())
		return ctrl.Result{}, err
	}

	rLog.Debug(
		fmt.Sprintf(
			"Updating %s %s/%s with patch operation",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		),
	)
	before := current.DeepCopyObject().(client.Object)
	updateFields(current, desired)

	if patchErr := k8sClient.Patch(ctx, current, client.MergeFrom(before)); patchErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to patch %s %s/%s",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(current, &errorReason, nil, kind, desired.GetName())
		return ctrl.Result{}, patchErr
	}

	successMessage := fmt.Sprintf(
		"Successfully updated %s %s/%s.",
		kind,
		desired.GetNamespace(),
		desired.GetName(),
	)
	scope.ReplaceDescendant(current, nil, &successMessage, kind, desired.GetName())
	return ctrl.Result{}, nil
}

func ReconcileOnCreate[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	scheme *runtime.Scheme,
	scope *state.Scope,
	k8sClient client.Client,
	desired T,
) (ctrl.Result, error) {
	kind := reflect.TypeOf(desired).Elem().Name()

	rLog.Debug(
		fmt.Sprintf(
			"%s %s/%s does not exist",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		),
	)
	if controllerRefErr := ctrl.SetControllerReference(
		&scope.SecurityConfig,
		desired,
		scheme,
	); controllerRefErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to set ownerReference on %s %s/%s.",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, kind, desired.GetName())
		return ctrl.Result{}, controllerRefErr
	}

	rLog.Info(
		fmt.Sprintf("Creating %s %s/%s", kind, desired.GetNamespace(), desired.GetName()),
	)
	if createErr := k8sClient.Create(ctx, desired); createErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to create %s %s/%s",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, kind, desired.GetName())
		return ctrl.Result{}, createErr
	}
	successMessage := fmt.Sprintf(
		"Successfully created %s %s/%s.",
		kind,
		desired.GetNamespace(),
		desired.GetName(),
	)
	scope.ReplaceDescendant(desired, nil, &successMessage, kind, desired.GetName())

	return ctrl.Result{}, nil
}

func ReconcileOnDelete[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	scope *state.Scope,
	current T,
	currentIsOwnedBySecurityConfig bool,
) (ctrl.Result, error) {
	kind := current.GetObjectKind().GroupVersionKind().Kind

	rLog.Info(
		fmt.Sprintf(
			"Desired %s %s/%s is nil. Will try to delete it if it is owned by SecurityConfig %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		),
	)

	if !currentIsOwnedBySecurityConfig {
		err := fmt.Errorf(
			"%s %s/%s is not owned by SecurityConfig %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		)
		rLog.Error(
			err,
			"Cannot delete resource that is not owned by SecurityConfig.",
		)
		errMsg := fmt.Sprintf(
			"Cannot delete %s %s/%s as it is not owned by SecurityConfig %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		)
		scope.ReplaceDescendant(current, &errMsg, nil, kind, current.GetName())
		return ctrl.Result{}, err
	}

	rLog.Debug(
		fmt.Sprintf(
			"%s %s/%s is owned by SecurityConfig %s/%s. Will try to delete it.",
			kind,
			current.GetNamespace(),
			current.GetName(),
			scope.SecurityConfig.Namespace,
			scope.SecurityConfig.Name,
		),
	)
	if deleteErr := k8sClient.Delete(ctx, current); deleteErr != nil {
		deleteErrorMessage := fmt.Sprintf(
			"Failed to delete %s %s/%s",
			kind,
			current.GetNamespace(),
			current.GetName(),
		)
		rLog.Error(deleteErr, deleteErrorMessage)
		scope.ReplaceDescendant(current, &deleteErrorMessage, nil, kind, current.GetName())
		return ctrl.Result{}, deleteErr
	}

	rLog.Debug(
		fmt.Sprintf("Successfully deleted %s %s/%s", kind, current.GetNamespace(), current.GetName()),
	)
	return ctrl.Result{}, nil
}

func DetermineReconcileAction[T client.Object](
	current T,
	desired *T,
	isDesiredNil bool,
	shouldUpdate func(current, desired T) bool,
	currentExists bool,
	currentIsOwnedBySecurityConfig bool,
) (*ReconcileAction, error) {
	if isDesiredNil {
		if currentExists && currentIsOwnedBySecurityConfig {
			return utilities.Ptr(RequiresDeleteAction), nil
		}
		return utilities.Ptr(RequiresNoAction), nil
	}

	if !currentExists {
		return utilities.Ptr(RequiresCreateAction), nil
	}

	if !currentIsOwnedBySecurityConfig {
		return nil, fmt.Errorf(
			"cannot update %s/%s as it is not owned by SecurityConfig",
			current.GetNamespace(),
			current.GetName(),
		)
	}

	if shouldUpdate(current, *desired) {
		return utilities.Ptr(RequiresUpdateAction), nil
	}

	return utilities.Ptr(RequiresNoAction), nil
}
