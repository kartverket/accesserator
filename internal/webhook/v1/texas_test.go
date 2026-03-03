package v1_test

import (
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("pod_webhook.go unit tests", func() {
	Describe("GetTexasContainer", func() {
		It("builds a Texas init container with tokenx enabled then tokenx is enabled in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
			}
			c := v1.GetTexasContainer(securityConfig)
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s", config.Get().TexasImageName, config.Get().TexasImageTag)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: v1.TokenXEnabledEnvVarName, Value: "true"}))
			Expect(c.EnvFrom).NotTo(BeEmpty())
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
							},
						},
					},
				),
			)
		})
	})

	Describe("GetTexasEnvVars", func() {
		It("returns env vars with tokenx disabled and no integration secrets when tokenx is not configured in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			envVars := v1.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with tokenx disabled and no integration secrets when tokenx is disabled in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: false,
					},
				},
			}
			envVars := v1.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("false"))
			Expect(envVars.IntegrationSecretsRefs).To(BeEmpty())
		})

		It("returns env vars with tokenx enabled and the expected integration secret ref when tokenx is enabled in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
			}
			envVars := v1.GetTexasEnvVars(securityConfig)
			Expect(envVars.TokenXEnabled).To(Equal("true"))
			Expect(envVars.IntegrationSecretsRefs).To(ContainElement(
				corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
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
					ApplicationRef: "myapp",
				}}
			a := v1.GetTexasContainer(securityConfig)
			b := v1.GetTexasContainer(securityConfig)
			Expect(v1.IsTexasContainerEqual(a, b)).To(BeTrue())

			b.Env = append(b.Env, corev1.EnvVar{Name: "DUMMY_ENV_VAR", Value: "dummy"})
			Expect(v1.IsTexasContainerEqual(a, b)).To(BeFalse())
		})
	})

	Describe("MutatePodWithTexasInitContainer", func() {
		It("returns an error when the pod already has a texas init container", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: "myapp",
				},
			}
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						v1.GetTexasContainer(securityConfig),
					},
				},
			}
			Expect(v1.MutatePodWithTexasInitContainer(pod, v1.GetTexasContainer(securityConfig))).To(MatchError(fmt.Sprintf("pod already has an init container named %s", v1.TexasInitContainerName)))
		})

		It("mutates the pod with the Texas init container when no init container with the same name exists", func() {
			appName := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: appName,
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: appName},
					},
				},
			}
			Expect(v1.MutatePodWithTexasInitContainer(&pod, v1.GetTexasContainer(securityConfig))).To(Succeed())
			Expect(pod.Spec.InitContainers).To(HaveLen(1))
			Expect(v1.IsTexasContainerEqual(v1.GetTexasContainer(securityConfig), pod.Spec.InitContainers[0])).To(BeTrue())
		})
	})

	Describe("MutatePodWithTexasURLEnvVar", func() {
		It("returns an error when the target container already has the Texas URL env var", func() {
			appName := "myapp"
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: appName,
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: v1.GetTexasUrlEnvVarValue()},
							},
						},
					},
				},
			}
			Expect(v1.MutatePodWithTexasURLEnvVar(&pod, appName)).To(MatchError(fmt.Sprintf("container %s already has env var %s", appName, config.Get().TexasUrlEnvVarName)))
		})

		It("mutates the pod with the Texas URL env var when the target container does not already have it", func() {
			appName := "myapp"
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: appName},
					},
				},
			}
			Expect(v1.MutatePodWithTexasURLEnvVar(&pod, appName)).To(Succeed())
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: config.Get().TexasUrlEnvVarName, Value: v1.GetTexasUrlEnvVarValue()}))
		})
	})

	Describe("ValidateTexasInitContainer", func() {
		It("returns an error when the Texas init container is missing", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: "myapp",
				},
			}
			texasInitContainer := v1.GetTexasContainer(securityConfig)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
				},
			}
			Expect(v1.ValidateTexasInitContainer(pod, texasInitContainer)).
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
					ApplicationRef: "myapp",
				},
			}
			texasInitContainer := v1.GetTexasContainer(securityConfig)
			incorrectlyConfiguredContainer := texasInitContainer
			incorrectlyConfiguredContainer.Image = "incorrect-image"
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{incorrectlyConfiguredContainer},
				},
			}
			Expect(v1.ValidateTexasInitContainer(pod, texasInitContainer)).
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
					ApplicationRef: "myapp",
				},
			}
			texasInitContainer := v1.GetTexasContainer(securityConfig)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{texasInitContainer},
				},
			}
			Expect(v1.ValidateTexasInitContainer(pod, texasInitContainer)).To(Succeed())
		})
	})

	Describe("ValidateTexasURLEnvVar", func() {
		It("returns an error when the Texas URL env var is missing", func() {
			applicationRef := "myapp"
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
			Expect(v1.ValidateTexasURLEnvVar(pod, securityConfig)).
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
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
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
			Expect(v1.ValidateTexasURLEnvVar(pod, securityConfig)).
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
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: v1.GetTexasUrlEnvVarValue()},
							},
						},
					},
				},
			}
			Expect(v1.ValidateTexasURLEnvVar(pod, securityConfig)).To(Succeed())
		})
	})
})
