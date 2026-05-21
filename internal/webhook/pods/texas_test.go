package pods_test

import (
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/webhook/pods"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("pod_webhook.go unit tests", func() {

	const (
		applicationRef     = "myapp"
		securityConfigName = "myconfig"
	)

	Describe("GetTexasContainer", func() {
		It("builds a Texas init container with tokenx enabled then tokenx is enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					JwkerSecretName: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
				},
			}
			c := pods.GetTexasContainer(securityConfig)
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s@%s", config.Get().TexasImageName, config.Get().TexasImageTag, config.Get().TexasImageSha)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: pods.TokenXEnabledEnvVarName, Value: "true"}))
			Expect(c.EnvFrom).NotTo(BeEmpty())
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: securityConfig.Status.JwkerSecretName,
							},
						},
					},
				),
			)
		})

		It("builds a Texas init container with maskinporten enabled then maskinporten is enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					MaskinportenSecretName: utilities.GetMaskinportenSecretName(securityConfigName),
				},
			}
			c := pods.GetTexasContainer(securityConfig)
			Expect(c.ReadinessProbe).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(c.ReadinessProbe.HTTPGet.Port).To(Equal(intstr.FromInt32(config.Get().TexasProbePort)))
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: pods.MaskinportenEnabledEnvVarName, Value: "true"}))
			Expect(c.EnvFrom).NotTo(BeEmpty())
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: securityConfig.Status.MaskinportenSecretName,
							},
						},
					},
				),
			)
		})

		It("builds a texas init container with both tokenx and maskinporten enabled when they are enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					JwkerSecretName:         utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
					MaskinportenSectretName: utilities.GetMaskinportenSecretName(securityConfigName),
				},
			}
			c := pods.GetTexasContainer(securityConfig)
			Expect(c.ReadinessProbe).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet).ToNot(BeNil())
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(c.ReadinessProbe.HTTPGet.Port).To(Equal(intstr.FromInt32(config.Get().TexasProbePort)))
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: pods.TokenXEnabledEnvVarName, Value: "true"}))
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: pods.MaskinportenEnabledEnvVarName, Value: "true"}))
			Expect(c.EnvFrom).NotTo(BeEmpty())
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: securityConfig.Status.JwkerSecretName,
							},
						},
					},
				),
			)
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: securityConfig.Status.MaskinportenSecretName,
							},
						},
					},
				),
			)
		})
	})

	Describe("GetTexasEnvVars", func() {
		It("returns env vars with tokenx disabled and no integration secrets when tokenx is not configured in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with tokenx disabled and no integration secrets when tokenx is disabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: false,
					},
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with tokenx enabled and the expected integration secret ref when tokenx is enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					JwkerSecretName: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("true"))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.JwkerSecretName,
						},
					},
				},
			))
		})

		It("returns env vars with maskinporten disabled and no integration secrets when maskinporten is not configured in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.MaskinportenEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with maskinporten disabled and no integration secrets when maskinporten is disabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: false,
					},
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.MaskinportenEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with maskinporten enabled and the expected integration secret ref when maskinporten is enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					MaskinportenSecretName: utilities.GetMaskinportenSecretName(securityConfigName),
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.MaskinportenEnabled).To(Equal("true"))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.MaskinportenSecretName,
						},
					},
				},
			))
		})

		It("returns env vars with maskinporten and tokenx enabled and the expected integration secret refs when both are enabled in SecurityConfig", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					MaskinportenSectretName: utilities.GetMaskinportenSecretName(securityConfigName),
					JwkerSecretName:         utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.MaskinportenEnabled).To(Equal("true"))
			Expect(envVars.TokenXEnabled).To(Equal("true"))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.MaskinportenSectretName,
						},
					},
				},
			))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.JwkerSecretName,
						},
					},
				},
			))
		})

		It("returns env vars with maskinporten enabled and tokenx disabled and the expected integration secret ref for only for maskinporten", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: false,
					},
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
				},
				Status: v1alpha.SecurityConfigStatus{
					MaskinportenSectretName: utilities.GetMaskinportenSecretName(securityConfigName),
					JwkerSecretName:         utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
				},
			}
			envVars := pods.GetTexasEnvVars(securityConfig)
			Expect(envVars.MaskinportenEnabled).To(Equal("true"))
			Expect(envVars.TokenXEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.MaskinportenSecretName,
						},
					},
				},
			))
			Expect(envVars.IntegrationSecretsRefs).ToNot(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: securityConfig.Status.JwkerSecretName,
						},
					},
				},
			))
		})
	})

	Describe("IsTexasContainerEqual", func() {
		It("returns true for identical containers and false when a field differs", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					Maskinporten: &v1alpha.MaskinportenSpec{
						Enabled: true,
						Client: &v1alpha.MaskinportenClientSpec{
							ClientName: securityConfigName,
							Scopes: &v1alpha.MaskinportenScope{
								ConsumedScopes: []naisiov1.ConsumedScope{
									{Name: "scope1"},
									{Name: "scope2"},
								},
							},
						},
					},
					ApplicationRef: applicationRef,
				}}
			a := pods.GetTexasContainer(securityConfig)
			b := pods.GetTexasContainer(securityConfig)
			Expect(pods.IsTexasContainerEqual(a, b)).To(BeTrue())

			b.Env = append(b.Env, corev1.EnvVar{Name: "DUMMY_ENV_VAR", Value: "dummy"})
			Expect(pods.IsTexasContainerEqual(a, b)).To(BeFalse())
		})
	})

	Describe("MutatePodWithTexasInitContainer", func() {
		It("returns an error when the pod already has a texas init container", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						pods.GetTexasContainer(securityConfig),
					},
				},
			}
			Expect(pods.MutatePodWithTexasInitContainer(pod, pods.GetTexasContainer(securityConfig))).To(MatchError(fmt.Sprintf("pod already has a container named %s", pods.TexasInitContainerName)))
		})

		It("returns an error when the pod already has container on texas port", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "dummy",
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: config.Get().TexasPort,
							},
						},
					}},
				},
			}

			sidecarContainer := pods.GetTexasContainer(securityConfig)
			Expect(pods.MutatePodWithTexasInitContainer(pod, sidecarContainer)).
				To(MatchError(fmt.Sprintf("pod already has a port on %d", sidecarContainer.Ports[0].ContainerPort)))
		})

		It("returns an error when the pod already has container on texas port or probe port", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			sidecarContainer := pods.GetTexasContainer(securityConfig)

			podWithTexasPort := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "dummy",
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: config.Get().TexasPort,
							},
						},
					}},
				},
			}
			Expect(pods.MutatePodWithTexasInitContainer(podWithTexasPort, sidecarContainer)).
				To(MatchError(fmt.Sprintf("pod already has a port on %d", sidecarContainer.Ports[0].ContainerPort)))

			podWithTexasProbePort := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "dummy",
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: config.Get().TexasProbePort,
							},
						},
					}},
				},
			}
			Expect(pods.MutatePodWithTexasInitContainer(podWithTexasProbePort, sidecarContainer)).
				To(MatchError(fmt.Sprintf("pod already has a port on %d", sidecarContainer.Ports[1].ContainerPort)))
		})

		It("mutates the pod with the Texas init container when no init container with the same name exists", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: applicationRef},
					},
				},
			}
			Expect(pods.MutatePodWithTexasInitContainer(&pod, pods.GetTexasContainer(securityConfig))).To(Succeed())
			Expect(pod.Spec.InitContainers).To(HaveLen(1))
			Expect(pods.IsTexasContainerEqual(pods.GetTexasContainer(securityConfig), pod.Spec.InitContainers[0])).To(BeTrue())
		})
	})

	Describe("MutatePodWithTexasURLEnvVar", func() {
		It("returns an error when the target container already has the Texas URL env var", func() {
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: pods.GetTexasUrlEnvVarValue()},
							},
						},
					},
				},
			}
			Expect(pods.MutatePodWithTexasURLEnvVar(&pod, applicationRef)).To(MatchError(fmt.Sprintf("container %s already has env var %s", applicationRef, config.Get().TexasUrlEnvVarName)))
		})

		It("mutates the pod with the Texas URL env var when the target container does not already have it", func() {
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: applicationRef},
					},
				},
			}
			Expect(pods.MutatePodWithTexasURLEnvVar(&pod, applicationRef)).To(Succeed())
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: config.Get().TexasUrlEnvVarName, Value: pods.GetTexasUrlEnvVarValue()}))
		})
	})

	Describe("ValidateTexasInitContainer", func() {
		It("returns an error when the Texas init container is missing", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			texasInitContainer := pods.GetTexasContainer(securityConfig)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
				},
			}
			Expect(pods.ValidateTexasInitContainer(pod, texasInitContainer)).
				To(
					MatchError(
						fmt.Sprintf(
							"pod is annotated to have Texas, but Texas init container is missing or not correctly configured for pod %s/%s",
							pod.Namespace,
							pod.Name,
						),
					),
				)
		})

		It("returns an error when the Texas init container is present but not correctly configured", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			texasInitContainer := pods.GetTexasContainer(securityConfig)
			incorrectlyConfiguredContainer := texasInitContainer
			incorrectlyConfiguredContainer.Image = "incorrect-image"
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{incorrectlyConfiguredContainer},
				},
			}
			Expect(pods.ValidateTexasInitContainer(pod, texasInitContainer)).
				To(
					MatchError(
						fmt.Sprintf(
							"pod is annotated to have Texas, but Texas init container is missing or not correctly configured for pod %s/%s",
							pod.Namespace,
							pod.Name,
						),
					),
				)
		})

		It("succeeds when the Texas init container is present and correctly configured", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			texasInitContainer := pods.GetTexasContainer(securityConfig)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{texasInitContainer},
				},
			}
			Expect(pods.ValidateTexasInitContainer(pod, texasInitContainer)).To(Succeed())
		})
	})

	Describe("ValidateTexasURLEnvVar", func() {
		It("returns an error when the Texas URL env var is missing", func() {
			applicationRef := applicationRef
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: v1alpha.ResourceName(applicationRef),
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: applicationRef},
					},
				},
			}
			Expect(pods.ValidateTexasURLEnvVar(pod, securityConfig)).
				To(
					MatchError(
						fmt.Sprintf(
							"pod is annotated to have Texas, but %s env var is either missing or not correct for pod %s/%s",
							config.Get().TexasUrlEnvVarName,
							pod.Namespace,
							pod.Name,
						),
					),
				)
		})

		It("returns an error when the Texas URL env var is present but has the wrong value", func() {
			applicationRef := applicationRef
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: v1alpha.ResourceName(applicationRef),
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: "incorrect-value"},
							},
						},
					},
				},
			}
			Expect(pods.ValidateTexasURLEnvVar(pod, securityConfig)).
				To(
					MatchError(
						fmt.Sprintf(
							"pod is annotated to have Texas, but %s env var is either missing or not correct for pod %s/%s",
							config.Get().TexasUrlEnvVarName,
							pod.Namespace,
							pod.Name,
						),
					),
				)
		})

		It("succeeds when the Texas URL env var is present and has the correct value", func() {
			applicationRef := applicationRef
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: v1alpha.ResourceName(applicationRef),
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: pods.GetTexasUrlEnvVarValue()},
							},
						},
					},
				},
			}
			Expect(pods.ValidateTexasURLEnvVar(pod, securityConfig)).To(Succeed())
		})
	})
})
