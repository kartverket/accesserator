package v1alpha

import (
	"context"

	"github.com/kartverket/accesserator/internal/resolver"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
)

// nolint:unused
var securityconfiglog = logf.Log.WithName("securityconfig-webhook")

// SetupSecurityConfigWebhookWithManager registers the webhook for SecurityConfig in the manager.
func SetupSecurityConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &accesseratorv1alpha.SecurityConfig{}).
		WithValidator(&SecurityConfigCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-accesserator-kartverket-no-v1alpha-securityconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=accesserator.kartverket.no,resources=securityconfigs,verbs=create;update,versions=v1alpha,name=vsecurityconfig-v1alpha.kb.io,admissionReviewVersions=v1

// SecurityConfigCustomValidator struct is responsible for validating the SecurityConfig resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SecurityConfigCustomValidator struct{}

var _ admission.Validator[*accesseratorv1alpha.SecurityConfig] = &SecurityConfigCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SecurityConfig.
func (v *SecurityConfigCustomValidator) ValidateCreate(_ context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon creation", "name", securityConfig.GetName())
	return validateSecurityConfig(securityConfig)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SecurityConfig.
func (v *SecurityConfigCustomValidator) ValidateUpdate(_ context.Context, _, newSecurityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon update", "name", newSecurityConfig.GetName())
	return validateSecurityConfig(newSecurityConfig)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SecurityConfig.
func (v *SecurityConfigCustomValidator) ValidateDelete(_ context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon deletion", "name", securityConfig.GetName())
	securityconfiglog.Info("Currently no validation logic implemented for SecurityConfig deletion")
	return nil, nil
}

func validateSecurityConfig(securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	if securityConfig.Spec.Opa != nil {
		if err := resolver.ValidateBundleURLs(securityConfig.Spec.Opa.BundleURLs); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
