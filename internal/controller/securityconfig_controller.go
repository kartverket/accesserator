/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecurityConfigReconciler reconciles a SecurityConfig object
type SecurityConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecurityConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *SecurityConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rlog := log.GetLogger(ctx)
	securityConfig := new(accesseratorv1alpha.SecurityConfig)
	rlog.Info("Reconciling SecurityConfig", "name", req.NamespacedName)

	if err := r.Client.Get(ctx, req.NamespacedName, securityConfig); err != nil {
		if apierrors.IsNotFound(err) {
			rlog.Debug(
				fmt.Sprintf("SecurityConig with name %s not found. Probably a delete.", req.NamespacedName.String()),
			)
			return reconcile.Result{}, nil
		}
		rlog.Error(err, fmt.Sprintf("Failed to get SecurityConfig with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(
		securityConfig,
		"Normal",
		"ReconcileStarted",
		fmt.Sprintf("SecurityConfig with name %s started.", req.NamespacedName.String()),
	)
	rlog.Debug(fmt.Sprintf("SecurityConfig with name %s found", req.NamespacedName.String()))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecurityConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accesseratorv1alpha.SecurityConfig{}).
		Named("securityconfig").
		Complete(r)
}
