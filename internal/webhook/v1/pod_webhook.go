package v1

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	SkiperatorApplicationRefLabel     = "application.skiperator.no/app-name"
	AccesseratorServicesAnnotation    = "accesserator.kartverket.no/services"
	AccesseratorVerifyAnnotationKey   = "accesserator.kartverket.no/verify"
	AccesseratorVerifyAnnotationValue = "true"
	TexasInitContainerName            = "texas"
	TexasPortName                     = "http"

	MaskinportenEnabledEnvVarName = "MASKINPORTEN_ENABLED"
	AzureEnabledEnvVarName        = "AZURE_ENABLED"
	IdportenEnabledEnvVarName     = "IDPORTEN_ENABLED"
	TokenXEnabledEnvVarName       = "TOKEN_X_ENABLED"

	Texas ServiceType = iota
)

type PodSecurityConfiguration struct {
	SecurityConfig                   v1alpha.SecurityConfig
	AppName                          string
	CreatedFromSkiperatorApplication bool
	AccesseratorServices             []AccesseratorService
}

type AccesseratorService struct {
	ServiceType  ServiceType
	Container    corev1.Container
	ValidateFunc func(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error
}

type ServiceType int

func (serviceType ServiceType) String() string {
	switch serviceType {
	case Texas:
		return TexasInitContainerName
	default:
		return "unknown"
	}
}

type TexasEnvVars struct {
	TokenXEnabled       string
	MaskinportenEnabled string
	AzureEnabled        string
	IdportenEnabled     string
	IntegrationSecrets  []corev1.EnvFromSource
}

// nolint:unused
// log is for logging in this package.
var podlog = logf.Log.WithName("pod-webhook")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1.Pod{}).
		WithValidator(&PodCustomValidator{Client: mgr.GetClient()}).
		WithDefaulter(&PodCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=mpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pod when those are created or updated.
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
		return nil
	}

	for _, accesseratorService := range podSecurityConfig.AccesseratorServices {
		switch accesseratorService.ServiceType {
		case Texas:
			for i := range pod.Spec.Containers {
				if pod.Spec.Containers[i].Name == podSecurityConfig.AppName {
					pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
						Name:  config.Get().TexasUrlEnvVarName,
						Value: GetTexasUrlEnvVarValue(),
					})
					break
				}
			}
			podlog.Info(fmt.Sprintf("Injecting %s", accesseratorService.ServiceType.String()))
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, accesseratorService.Container)

		default:
			return fmt.Errorf("failed to mutate pod with accesserator service '%s'", accesseratorService.ServiceType.String())
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator struct is responsible for validating the Pod resource
// when it is created, updated, or deleted.
type PodCustomValidator struct {
	Client client.Client
}

var _ admission.Validator[*corev1.Pod] = &PodCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateCreate(ctx context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, pod)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateUpdate(ctx context.Context, _, newPod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, newPod)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateDelete(_ context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validation for Pod upon deletion", "name", pod.GetName())

	// Nothing to do

	return nil, nil
}

// GetPodSecurityConfiguration returns the PodSecurityConfiguration for a given pod.
// The PodSecurityConfiguration holds the SecurityConfig for a pod, which Skiperator application the pod stems from
// and which services (Texas, opa, etc.) the pod should have.
// Returns an error if the SecurityConfig cannot be found.
func GetPodSecurityConfiguration(ctx context.Context, k8sClient client.Client, pod *corev1.Pod) (*PodSecurityConfiguration, error) {
	if pod.Labels == nil {
		return &PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}
	appName, appNameExists := pod.Labels[SkiperatorApplicationRefLabel]
	if !appNameExists {
		return &PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}

	if k8sClient == nil {
		return nil, fmt.Errorf("webhook client is not configured")
	}

	accesseratorVerifyAnnotationValue, accesseratorVerifyAnnotationExists := pod.Annotations[AccesseratorVerifyAnnotationKey]
	accesseratorServicesAnnotationValue, accesseratorServicesAnnotationExists := pod.Annotations[AccesseratorServicesAnnotation]
	var accesseratorServiceTypes []ServiceType
	if accesseratorServicesAnnotationExists {
		accesseratorServiceTypes = ParseAccesseratorServices(accesseratorServicesAnnotationValue)
	}

	var securityConfig v1alpha.SecurityConfig
	if (accesseratorVerifyAnnotationExists && accesseratorVerifyAnnotationValue == AccesseratorVerifyAnnotationValue) || len(accesseratorServiceTypes) > 0 {
		// The pod want to verify the correct existence of a SecurityConfig AND/OR the pod has an annotation specifying which Accesserator services it wants.
		// Either case, we need to fetch the SecurityConfig.
		securityConfigForApplication, getSecurityConfigErr := GetSecurityConfigForApplication(ctx, k8sClient, types.NamespacedName{Namespace: pod.Namespace, Name: appName})
		if getSecurityConfigErr != nil {
			return nil, getSecurityConfigErr
		}
		securityConfig = *securityConfigForApplication
	}

	accesseratorServices := make([]AccesseratorService, 0, len(accesseratorServiceTypes))
	for _, serviceType := range accesseratorServiceTypes {
		switch serviceType {
		case Texas:
			texasContainer := GetTexasContainer(securityConfig)
			serviceValidationFunc, getServiceValidationFuncErr := GetServiceValidationFunc(Texas, &texasContainer)
			if getServiceValidationFuncErr != nil {
				return nil, getServiceValidationFuncErr
			}
			accesseratorServices = append(
				accesseratorServices,
				AccesseratorService{
					ServiceType:  Texas,
					Container:    texasContainer,
					ValidateFunc: serviceValidationFunc,
				},
			)
		default:
			return nil, fmt.Errorf("pod annotated with unknown accesserator service type: %s", serviceType)
		}
	}

	return &PodSecurityConfiguration{
		SecurityConfig:                   securityConfig,
		AppName:                          appName,
		CreatedFromSkiperatorApplication: true,
		AccesseratorServices:             accesseratorServices,
	}, nil
}

func GetTexasContainer(securityConfig v1alpha.SecurityConfig) corev1.Container {
	texasImageUrl := fmt.Sprintf(
		"%s:%s",
		config.Get().TexasImageName,
		config.Get().TexasImageTag,
	)
	texasEnvVars := GetTexasEnvVars(securityConfig)
	return corev1.Container{
		Name:  TexasInitContainerName,
		Image: texasImageUrl,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: config.Get().TexasPort,
				Name:          TexasPortName,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		// NOTE: RestartPolicy Always is only available for init containers in Kubernetes v1.33+
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
		RestartPolicy: utilities.Ptr(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: utilities.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
				Add: []corev1.Capability{
					"NET_BIND_SERVICE",
				},
			},
			Privileged:             utilities.Ptr(false),
			ReadOnlyRootFilesystem: utilities.Ptr(true),
			RunAsGroup:             utilities.Ptr(int64(150)),
			RunAsNonRoot:           utilities.Ptr(true),
			RunAsUser:              utilities.Ptr(int64(150)),
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env: []corev1.EnvVar{
			{
				Name:  TokenXEnabledEnvVarName,
				Value: texasEnvVars.TokenXEnabled,
			},
			{
				Name:  MaskinportenEnabledEnvVarName,
				Value: texasEnvVars.MaskinportenEnabled,
			},
			{
				Name:  AzureEnabledEnvVarName,
				Value: texasEnvVars.AzureEnabled,
			},
			{
				Name:  IdportenEnabledEnvVarName,
				Value: texasEnvVars.IdportenEnabled,
			},
		},
		EnvFrom: texasEnvVars.IntegrationSecrets,
	}
}

func GetTexasEnvVars(securityConfig v1alpha.SecurityConfig) TexasEnvVars {
	var integrationSecrets []corev1.EnvFromSource
	tokenxEnabled := "false"
	if securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled {
		tokenxEnabled = "true"
		integrationSecrets = append(integrationSecrets, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(securityConfig.Spec.ApplicationRef)),
				},
			},
		})
	}
	return TexasEnvVars{
		TokenXEnabled:       tokenxEnabled,
		MaskinportenEnabled: "false",
		AzureEnabled:        "false",
		IdportenEnabled:     "false",
		IntegrationSecrets:  integrationSecrets,
	}
}

func validatePod(ctx context.Context, k8sClient client.Client, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validating for Pod", "name", pod.GetName())

	podSecurityConfig, getSecurityConfigForPodErr := GetPodSecurityConfiguration(ctx, k8sClient, pod)
	if getSecurityConfigForPodErr != nil {
		podlog.Error(getSecurityConfigForPodErr, "Failed to validate for Pod")
		return nil, getSecurityConfigForPodErr
	}
	if !podSecurityConfig.CreatedFromSkiperatorApplication {
		return nil, nil
	}

	// Validate that each accesserator service is configured correctly
	for _, accesseratorService := range podSecurityConfig.AccesseratorServices {
		if err := accesseratorService.ValidateFunc(*pod, podSecurityConfig.SecurityConfig); err != nil {
			podlog.Error(err, "Failed to validate for Pod")
			return nil, err
		}
	}

	return nil, nil
}

func IsTexasContainerEqual(expected, actual corev1.Container) bool {
	return expected.Name == actual.Name &&
		expected.Image == actual.Image &&
		reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) &&
		reflect.DeepEqual(expected.Env, actual.Env) &&
		reflect.DeepEqual(expected.EnvFrom, actual.EnvFrom) &&
		reflect.DeepEqual(expected.Ports, actual.Ports) &&
		reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) &&
		reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) &&
		reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy)
}

func GetTexasUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().TexasPort)
}

func GetSecurityConfigForApplication(ctx context.Context, k8sClient client.Client, applicationObjectKey client.ObjectKey) (*v1alpha.SecurityConfig, error) {
	var securityConfigList v1alpha.SecurityConfigList
	podlog.Info("Fetching SecurityConfig resources")
	if err := k8sClient.List(ctx, &securityConfigList, client.InNamespace(applicationObjectKey.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to fetch SecurityConfig resources: %w", err)
	}

	var securityConfigForApplication []v1alpha.SecurityConfig
	for _, securityConfig := range securityConfigList.Items {
		if securityConfig.Spec.ApplicationRef == applicationObjectKey.Name {
			securityConfigForApplication = append(securityConfigForApplication, securityConfig)
		}
	}

	if len(securityConfigForApplication) < 1 {
		msg := "no SecurityConfig resource was found for the corresponding Application"
		podlog.Info(msg, "namespacedName", applicationObjectKey)
		return nil, fmt.Errorf("%s", msg)
	}

	if len(securityConfigForApplication) > 1 {
		msg := "multiple SecurityConfig resources found for Application"
		podlog.Info(msg, "namespacedName", applicationObjectKey)
		return nil, fmt.Errorf("%s", msg)
	}

	securityConfig := &securityConfigForApplication[0]

	if securityConfig == nil {
		msg := "SecurityConfig resource for Application was nil"
		podlog.Info(msg, "namespacedName", applicationObjectKey)
		return nil, fmt.Errorf("%s", msg)
	}

	if !securityConfig.Status.Ready {
		msg := "SecurityConfig resource for Application is not ready"
		podlog.Info(msg, "namespacedName", applicationObjectKey)
		return nil, fmt.Errorf("%s", msg)
	}

	return securityConfig, nil
}

func ParseAccesseratorServices(annotationValue string) []ServiceType {
	var services []ServiceType
	for _, s := range strings.Split(annotationValue, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		switch strings.ToLower(trimmed) {
		case Texas.String():
			services = append(services, Texas)
		}
	}
	return services
}

func GetServiceValidationFunc(serviceType ServiceType, sidecarContainer *corev1.Container) (func(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error, error) {
	switch serviceType {
	case Texas:
		return func(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
			// Validate that TEXAS_URL env var has the correct value
			hasTexasUrlEnvVar := false
			for _, container := range pod.Spec.Containers {
				if container.Name == securityConfig.Spec.ApplicationRef {
					for _, envVar := range container.Env {
						if envVar.Name == config.Get().TexasUrlEnvVarName && envVar.Value == GetTexasUrlEnvVarValue() {
							hasTexasUrlEnvVar = true
							break
						}
					}
					break
				}
			}
			if !hasTexasUrlEnvVar {
				return fmt.Errorf(
					"pod is annotated to have Texas but %s env var is missing for pod with name %s/%s",
					config.Get().TexasUrlEnvVarName,
					pod.Namespace,
					pod.Name,
				)
			}

			// Validate that the Texas init container is correctly configured
			texasContainerCorrectlyConfigured := false
			if sidecarContainer != nil {
				for _, initContainer := range pod.Spec.InitContainers {
					if initContainer.Name == TexasInitContainerName {
						if IsTexasContainerEqual(
							*sidecarContainer,
							initContainer,
						) {
							texasContainerCorrectlyConfigured = true
						}
						break
					}
				}
			}
			if !texasContainerCorrectlyConfigured {
				return fmt.Errorf(
					"pod is annotated to have Texas, but Texas init container is missing or not correctly configured for pod with name %s/%s",
					pod.Namespace,
					pod.Name,
				)
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unknown service type '%s'", serviceType.String())
	}
}
