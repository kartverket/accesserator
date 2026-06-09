package securityconfigs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// signatureValidationTimeout caps how long an admission request will wait for
// signature checks (resolve manifest + fetch attestation + Rekor lookup).
// Kept well under the default webhook timeout of 10s.
const signatureValidationTimeout = 8 * time.Second

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
func (v *SecurityConfigCustomValidator) ValidateCreate(ctx context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon creation", "name", securityConfig.GetName())
	return validateSecurityConfig(ctx, securityConfig)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SecurityConfig.
func (v *SecurityConfigCustomValidator) ValidateUpdate(ctx context.Context, _, newSecurityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon update", "name", newSecurityConfig.GetName())
	return validateSecurityConfig(ctx, newSecurityConfig)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SecurityConfig.
func (v *SecurityConfigCustomValidator) ValidateDelete(_ context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	securityconfiglog.Info("Validation for SecurityConfig upon deletion", "name", securityConfig.GetName())
	securityconfiglog.Info("Currently no validation logic implemented for SecurityConfig deletion")
	return nil, nil
}

func validateSecurityConfig(ctx context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) (admission.Warnings, error) {
	logger := log.GetLogger(ctx)
	if securityConfig.Spec.Opa == nil {
		return nil, nil
	}

	if !config.Get().OpaEnabled {
		return nil, fmt.Errorf("OPA is not enabled on this cluster and 'spec.opa' can therefore not be set")
	}

	if err := validation.ValidateBundleUrls(securityConfig.Spec.Opa.BundleURLs); err != nil {
		logger.Info(
			"SecurityConfig blocked by validating webhook",
			"validationError", err.Error(),
		)
		return nil, err
	}

	if err := validateBundleSignatures(ctx, securityConfig.Spec.Opa.BundleURLs); err != nil {
		logger.Info(
			"SecurityConfig blocked by validating webhook",
			"validationError", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

func validateBundleSignatures(ctx context.Context, bundles []accesseratorv1alpha.BundleSource) error {
	logger := log.GetLogger(ctx)

	// Skip the credential store setup if nothing in the spec asks for verification.
	hasVerification := false
	for _, b := range bundles {
		if b.Verification != nil {
			hasVerification = true
			break
		}
	}
	if !hasVerification {
		return nil
	}

	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		logger.Error(err, "Failed to create credential store for signature validation")
		return errors.New("failed to create credential store for signature validation")
	}

	for _, bundleSource := range bundles {
		if bundleSource.Verification == nil {
			continue
		}
		signatureCtx, cancel := context.WithTimeout(ctx, signatureValidationTimeout)
		validateSignatureErr := validation.ValidateBundleSourceSignature(signatureCtx, validation.DefaultAttestationFetcher, credStore, bundleSource)
		cancel()
		if validateSignatureErr != nil {
			return validateSignatureErr
		}
	}
	return nil
}
