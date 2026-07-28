package securityconfigs

import (
	"context"
	"fmt"
	"time"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

const opaBundleVerificationTimeout = 30 * time.Second

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

	if securityConfig.Spec.Tokenx != nil && !config.Get().TokenxEnabled {
		return nil, fmt.Errorf("TokenX is not enabled on this cluster and 'spec.tokenx' can therefore not be set")
	}
	if securityConfig.Spec.Opa != nil {
		if !config.Get().OpaEnabled {
			return nil, fmt.Errorf("OPA is not enabled on this cluster and 'spec.opa' can therefore not be set")
		}
		if validateOpaErr := validateOpa(logger, ctx, securityConfig); validateOpaErr != nil {
			return nil, validateOpaErr
		}
	}

	return nil, nil
}

func validateOpa(logger log.Logger, ctx context.Context, securityConfig *accesseratorv1alpha.SecurityConfig) error {
	logger.Debug("Validating SecurityConfig OPA bundle URL prefixes", "name", securityConfig.Name, "namespace", securityConfig.Namespace)
	if err := validation.ValidateBundleUrlPrefixes(securityConfig.Spec.Opa.BundleURLs); err != nil {
		logger.Warning(
			"SecurityConfig blocked by validating webhook",
			"name", securityConfig.Name, "namespace", securityConfig.Namespace, "validationError", err.Error(),
		)
		return err
	}
	logger.Debug("SecurityConfig OPA bundle URL prefixes validated successfully",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace,
	)

	logger.Debug("Verifying SecurityConfig OPA bundle URLs against source",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace,
	)
	if err := verifyBundleSignatures(logger, ctx, securityConfig.Spec.Opa.BundleURLs); err != nil {
		logger.Warning(
			"SecurityConfig blocked by validating webhook",
			"validationError", err.Error(),
		)
		return err
	}
	logger.Debug("SecurityConfig OPA bundle URLs verified successfully against source",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace,
	)
	return nil
}

// verifyBundleSignatures verifies the SLSA provenance attestation for every
// bundle that opts into verification.
func verifyBundleSignatures(logger log.Logger, ctx context.Context, bundles []accesseratorv1alpha.BundleSource) error {
	if !anyHasVerification(bundles) {
		logger.Debug("None of the bundles have verification configured, skipping signature verification")
		return nil
	}

	fetcher := validation.DefaultAttestationFetcher{}
	for _, bundle := range bundles {
		if bundle.Verification == nil {
			continue
		}
		if verifyErr := verifyBundle(ctx, fetcher, config.CredStore, bundle); verifyErr != nil {
			return verifyErr
		}
	}
	return nil
}

func verifyBundle(
	ctx context.Context,
	fetcher validation.AttestationFetcher,
	credStore credentials.Store,
	bundle accesseratorv1alpha.BundleSource,
) error {
	verifyCtx, cancel := context.WithTimeout(ctx, opaBundleVerificationTimeout)
	defer cancel()
	return validation.VerifyBundleSource(verifyCtx, fetcher, credStore, bundle)
}

func anyHasVerification(bundles []accesseratorv1alpha.BundleSource) bool {
	for _, bundle := range bundles {
		if bundle.Verification != nil {
			return true
		}
	}
	return false
}
