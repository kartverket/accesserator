package v1

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	SkiperatorApplicationRefLabel = "application.skiperator.no/app-name"

	AccesseratorServicesAnnotation    = "accesserator.kartverket.no/services"
	AccesseratorVerifyAnnotationKey   = "accesserator.kartverket.no/verify-securityconfig"
	AccesseratorVerifyAnnotationValue = "true"

	Texas ServiceType = iota
)

// ServiceType identifies a security sidecar service managed by Accesserator.
type ServiceType int

func (serviceType ServiceType) String() string {
	switch serviceType {
	case Texas:
		return TexasInitContainerName
	default:
		return "unknown"
	}
}

// AccesseratorService pairs a resolved sidecar container with its pod-mutation
// and validation logic.
type AccesseratorService struct {
	ServiceType  ServiceType
	Container    corev1.Container
	MutateFunc   func(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error
	ValidateFunc func(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error
}

// nolint:unused
var podlog = logf.Log.WithName("pod-webhook")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1.Pod{}).
		WithValidator(&PodCustomValidator{Client: mgr.GetClient()}).
		WithDefaulter(&PodCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=mpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomDefaulter is responsible for mutating Pods before they are persisted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PodCustomDefaulter struct {
	Client client.Client
}

var _ admission.Defaulter[*corev1.Pod] = &PodCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Pod.
func (d *PodCustomDefaulter) Default(ctx context.Context, pod *corev1.Pod) error {
	podlog.Info("Defaulting for Pod")

	podSecurityConfig, err := GetPodSecurityConfiguration(ctx, d.Client, pod)
	if err != nil {
		return err
	}
	if !podSecurityConfig.CreatedFromSkiperatorApplication {
		// Only mutate Pods that are created from Skiperator Applications.
		podlog.Info("Pod is not created from Skiperator Application, skipping mutation", "skiperatorAppName", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      podSecurityConfig.AppName,
		})
		return nil
	}

	if len(podSecurityConfig.AccesseratorServices) > 0 {
		for _, svc := range podSecurityConfig.AccesseratorServices {
			podlog.Info("Mutating Pod from Skiperator app", "app", types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      podSecurityConfig.AppName,
			}, "serviceType", svc.ServiceType.String())
			if mutateErr := svc.MutateFunc(pod, podSecurityConfig.SecurityConfig); mutateErr != nil {
				return mutateErr
			}
		}
	} else {
		podlog.Info("No Accesserator services to mutate for Pod", "pod", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		})
	}

	return nil
}

// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator is responsible for validating Pods on create and update.
type PodCustomValidator struct {
	Client client.Client
}

var _ admission.Validator[*corev1.Pod] = &PodCustomValidator{}

func (v *PodCustomValidator) ValidateCreate(ctx context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, pod)
}

func (v *PodCustomValidator) ValidateUpdate(ctx context.Context, _, newPod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, newPod)
}

func (v *PodCustomValidator) ValidateDelete(_ context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validation for Pod upon deletion", "name", pod.GetName())
	return nil, nil
}

func validatePod(ctx context.Context, k8sClient client.Client, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validating for Pod", "name", pod.GetName())

	podSecurityConfig, err := GetPodSecurityConfiguration(ctx, k8sClient, pod)
	if err != nil {
		return nil, err
	}
	if !podSecurityConfig.CreatedFromSkiperatorApplication {
		// Only validate Pods that are created from Skiperator Applications.
		podlog.Info("Pod is not created from Skiperator Application, skipping validation", "pod", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		})
		return nil, nil
	}

	if len(podSecurityConfig.AccesseratorServices) > 0 {
		for _, svc := range podSecurityConfig.AccesseratorServices {
			podlog.Info("Validating Pod", "pod", types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}, "serviceType", svc.ServiceType.String())
			if validateErr := svc.ValidateFunc(*pod, podSecurityConfig.SecurityConfig); validateErr != nil {
				return nil, validateErr
			}
		}
	} else {
		podlog.Info("No Accesserator services to validate for Pod", "pod", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		})
	}

	return nil, nil
}

// PodSecurityConfiguration holds all resolved security context for a Pod,
// used by both the mutating and validating webhooks.
type PodSecurityConfiguration struct {
	SecurityConfig                   v1alpha.SecurityConfig
	AppName                          string
	CreatedFromSkiperatorApplication bool
	AccesseratorServices             []AccesseratorService
}

// GetPodSecurityConfiguration resolves the full security configuration for a Pod.
// It returns a non-nil PodSecurityConfiguration in all non-error cases.
func GetPodSecurityConfiguration(ctx context.Context, k8sClient client.Client, pod *corev1.Pod) (*PodSecurityConfiguration, error) {
	if pod.Labels == nil {
		return &PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}

	appName, isSkiperatorPod := pod.Labels[SkiperatorApplicationRefLabel]
	if !isSkiperatorPod {
		return &PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}

	if k8sClient == nil {
		return nil, fmt.Errorf("webhook client is not configured")
	}

	verifyAnnotation, hasVerify := pod.Annotations[AccesseratorVerifyAnnotationKey]
	servicesAnnotation, hasServices := pod.Annotations[AccesseratorServicesAnnotation]

	shouldFetchSecurityConfig := (hasVerify && verifyAnnotation == AccesseratorVerifyAnnotationValue) || hasServices
	if !shouldFetchSecurityConfig {
		return &PodSecurityConfiguration{
			AppName:                          appName,
			CreatedFromSkiperatorApplication: true,
		}, nil
	}

	securityConfig, err := GetSecurityConfigForApplication(ctx, k8sClient, client.ObjectKey{
		Namespace: pod.Namespace,
		Name:      appName,
	})
	if err != nil {
		return nil, err
	}

	var serviceTypes []ServiceType
	if hasServices {
		serviceTypes = ParseAccesseratorServices(servicesAnnotation)
	}

	if len(serviceTypes) == 0 {
		return &PodSecurityConfiguration{
			SecurityConfig:                   *securityConfig,
			AppName:                          appName,
			CreatedFromSkiperatorApplication: true,
		}, nil
	}

	accesseratorServices, err := BuildAccesseratorServices(serviceTypes, *securityConfig)
	if err != nil {
		return nil, err
	}

	return &PodSecurityConfiguration{
		SecurityConfig:                   *securityConfig,
		AppName:                          appName,
		CreatedFromSkiperatorApplication: true,
		AccesseratorServices:             accesseratorServices,
	}, nil
}

// BuildAccesseratorServices builds the full AccesseratorService slice for the given service types.
func BuildAccesseratorServices(serviceTypes []ServiceType, securityConfig v1alpha.SecurityConfig) ([]AccesseratorService, error) {
	services := make([]AccesseratorService, 0, len(serviceTypes))
	for _, serviceType := range serviceTypes {
		expectedContainer, err := GetServiceContainer(serviceType, securityConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get expectedContainer for service type %s: %w", serviceType, err)
		}

		mutateFunc, err := GetServiceMutationFunc(serviceType)
		if err != nil {
			return nil, fmt.Errorf("failed to get mutation func for service type %s: %w", serviceType, err)
		}

		validateFunc, err := GetServiceValidationFunc(serviceType, *expectedContainer)
		if err != nil {
			return nil, fmt.Errorf("failed to get validation func for service type %s: %w", serviceType, err)
		}

		services = append(services, AccesseratorService{
			ServiceType:  serviceType,
			Container:    *expectedContainer,
			MutateFunc:   mutateFunc,
			ValidateFunc: validateFunc,
		})
	}
	return services, nil
}

// ParseAccesseratorServices parses the comma-separated services annotation value into
// a slice of recognised ServiceTypes, ignoring unknown values.
func ParseAccesseratorServices(annotationValue string) []ServiceType {
	var services []ServiceType
	for _, s := range strings.Split(annotationValue, ",") {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case Texas.String():
			if !slices.Contains(services, Texas) {
				services = append(services, Texas)
			}
		}
	}
	return services
}

// GetServiceContainer returns the resolved sidecar container for the given service type.
func GetServiceContainer(st ServiceType, securityConfig v1alpha.SecurityConfig) (*corev1.Container, error) {
	switch st {
	case Texas:
		c := GetTexasContainer(securityConfig)
		return &c, nil
	default:
		return nil, fmt.Errorf("unknown service type '%s'", st)
	}
}

// GetServiceValidationFunc returns the pod-validation function for the given service type.
func GetServiceValidationFunc(st ServiceType, expectedSidecarContainer corev1.Container) (func(corev1.Pod, v1alpha.SecurityConfig) error, error) {
	switch st {
	case Texas:
		return func(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
			return ValidateTexasOnPod(pod, securityConfig, expectedSidecarContainer)
		}, nil
	default:
		return nil, fmt.Errorf("unknown service type '%s'", st)
	}
}

// GetServiceMutationFunc returns the pod-mutation function for the given service type.
func GetServiceMutationFunc(st ServiceType) (func(*corev1.Pod, v1alpha.SecurityConfig) error, error) {
	switch st {
	case Texas:
		return func(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
			return MutateTexasOnPod(pod, securityConfig)
		}, nil
	default:
		return nil, fmt.Errorf("unknown service type '%s'", st)
	}
}
