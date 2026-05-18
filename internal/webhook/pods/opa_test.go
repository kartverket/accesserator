package pods_test

import (
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/webhook/pods"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("opa.go unit tests", func() {
	const (
		applicationRef     = "myapp"
		securityConfigName = "myconfig"
	)

	newSecurityConfig := func(opaEnabled bool) v1alpha.SecurityConfig {
		return v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: securityConfigName,
			},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: applicationRef,
				Opa: &v1alpha.OpenPolicyAgentSpec{
					Enabled: opaEnabled,
					BundleURLs: []v1alpha.BundleSource{
						{Name: "bundle-a", URL: "http://bundle-source/bundle-a:latest"},
						{Name: "bundle-b", URL: "http://bundle-source/bundle-b:latest"},
					},
				},
			},
			Status: v1alpha.SecurityConfigStatus{
				OpaBundleSource: &v1alpha.OpaBundleSource{
					ConfigMapName: utilities.GetOpaConfigMapName(securityConfigName),
					BundleNames:   []string{"bundle-a", "bundle-b"},
				},
			},
		}
	}

	Describe("GetOpaContainer", func() {
		It("builds an OPA init container without bundle args, watch, or volume mounts when OPA is disabled", func() {
			securityConfig := newSecurityConfig(false)

			c := pods.GetOpaContainer(securityConfig)

			Expect(c.Name).To(Equal(pods.OpaInitContainerName))
			Expect(c.Image).To(Equal(fmt.Sprintf(
				"%s:%s@%s",
				config.Get().OpaImageName,
				config.Get().OpaImageTag,
				config.Get().OpaImageSha,
			)))
			Expect(c.Command).To(Equal([]string{"opa"}))
			Expect(c.Args).To(Equal([]string{
				"run",
				"--server",
				fmt.Sprintf("--addr=0.0.0.0:%d", config.Get().OpaPort),
			}))
			Expect(c.Args).NotTo(ContainElement("--bundle"))
			Expect(c.Args).NotTo(ContainElement("--watch"))
			Expect(c.VolumeMounts).To(BeEmpty())
		})

		It("builds an OPA init container without bundle args, watch, or volume mounts when OPA is not configured", func() {
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: securityConfigName,
				},
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
				Status: v1alpha.SecurityConfigStatus{
					OpaBundleSource: &v1alpha.OpaBundleSource{
						ConfigMapName: utilities.GetOpaConfigMapName(securityConfigName),
						BundleNames:   []string{"bundle-a", "bundle-b"},
					},
				},
			}

			c := pods.GetOpaContainer(securityConfig)

			Expect(c.Command).To(Equal([]string{"opa"}))
			Expect(c.Args).To(Equal([]string{
				"run",
				"--server",
				fmt.Sprintf("--addr=0.0.0.0:%d", config.Get().OpaPort),
			}))
			Expect(c.Args).NotTo(ContainElement("--bundle"))
			Expect(c.Args).NotTo(ContainElement("--watch"))
			Expect(c.VolumeMounts).To(BeEmpty())
		})

		It("builds an OPA init container with bundle args from status when OPA is enabled", func() {
			securityConfig := newSecurityConfig(true)

			c := pods.GetOpaContainer(securityConfig)

			Expect(c.Name).To(Equal(pods.OpaInitContainerName))
			Expect(c.Image).To(Equal(fmt.Sprintf(
				"%s:%s@%s",
				config.Get().OpaImageName,
				config.Get().OpaImageTag,
				config.Get().OpaImageSha,
			)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Command).To(Equal([]string{"opa"}))
			Expect(c.Args).To(Equal([]string{
				"run",
				"--server",
				fmt.Sprintf("--addr=0.0.0.0:%d", config.Get().OpaPort),
				"--bundle",
				fmt.Sprintf("%s/%s", pods.OpaBundleMountPath, "bundle-a"),
				"--bundle",
				fmt.Sprintf("%s/%s", pods.OpaBundleMountPath, "bundle-b"),
				"--watch",
			}))
			Expect(c.Ports).To(ContainElement(corev1.ContainerPort{
				ContainerPort: config.Get().OpaPort,
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
			}))
			Expect(c.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      pods.OpaBundleVolumeName,
				MountPath: pods.OpaBundleMountPath,
				ReadOnly:  true,
			}))
		})
	})

	Describe("GetOpaUrlEnvVarValue", func() {
		It("returns the localhost OPA URL using the configured OPA port", func() {
			Expect(pods.GetOpaUrlEnvVarValue()).To(Equal(
				fmt.Sprintf("http://localhost:%d", config.Get().OpaPort),
			))
		})
	})

	Describe("IsOpaContainerEqual", func() {
		It("returns true for identical containers and false when a field differs", func() {
			securityConfig := newSecurityConfig(true)
			a := pods.GetOpaContainer(securityConfig)
			b := pods.GetOpaContainer(securityConfig)

			Expect(pods.IsOpaContainerEqual(a, b)).To(BeTrue())

			b.Args = append(b.Args, "--dummy")
			Expect(pods.IsOpaContainerEqual(a, b)).To(BeFalse())
		})

		It("allows extra volume mounts on the actual container", func() {
			securityConfig := newSecurityConfig(true)
			expected := pods.GetOpaContainer(securityConfig)
			actual := pods.GetOpaContainer(securityConfig)
			actual.VolumeMounts = append(actual.VolumeMounts, corev1.VolumeMount{
				Name:      "extra",
				MountPath: "/extra",
			})

			Expect(pods.IsOpaContainerEqual(expected, actual)).To(BeTrue())
		})
	})

	Describe("MutatePodWithOpaInitContainer", func() {
		It("returns an error when the pod already has an OPA init container", func() {
			securityConfig := newSecurityConfig(true)
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						pods.GetOpaContainer(securityConfig),
					},
				},
			}

			Expect(pods.MutatePodWithOpaInitContainer(pod, pods.GetOpaContainer(securityConfig))).
				To(MatchError(fmt.Sprintf("pod already has a container named %s", pods.OpaInitContainerName)))
		})

		It("mutates the pod with the OPA init container when no init container with the same name exists", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: applicationRef}},
				},
			}

			Expect(pods.MutatePodWithOpaInitContainer(&pod, pods.GetOpaContainer(securityConfig))).To(Succeed())
			Expect(pod.Spec.InitContainers).To(HaveLen(1))
			Expect(pods.IsOpaContainerEqual(pods.GetOpaContainer(securityConfig), pod.Spec.InitContainers[0])).To(BeTrue())
		})
	})

	Describe("MutatePodWithOpaURLEnvVar", func() {
		It("returns an error when the target container already has the OPA URL env var", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().OpaUrlEnvVarName, Value: pods.GetOpaUrlEnvVarValue()},
							},
						},
					},
				},
			}

			Expect(pods.MutatePodWithOpaURLEnvVar(&pod, securityConfig)).
				To(MatchError(fmt.Sprintf("container %s already has env var %s", applicationRef, config.Get().OpaUrlEnvVarName)))
		})

		It("mutates the pod with the OPA URL env var when the target container does not already have it", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: applicationRef}},
				},
			}

			Expect(pods.MutatePodWithOpaURLEnvVar(&pod, securityConfig)).To(Succeed())
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  config.Get().OpaUrlEnvVarName,
				Value: pods.GetOpaUrlEnvVarValue(),
			}))
		})
	})

	Describe("MutatePodWithOpaBundleVolume", func() {
		It("returns an error when the pod already has the OPA bundle volume", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						pods.GetOpaBundleVolume(securityConfig.Status.OpaBundleSource.ConfigMapName),
					},
				},
			}

			Expect(pods.MutatePodWithOpaBundleVolume(&pod, securityConfig)).
				To(MatchError(fmt.Sprintf("pod already has a volume named %s", pods.OpaBundleVolumeName)))
		})

		It("mutates the pod with the OPA bundle volume when no volume with the same name exists", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{}

			Expect(pods.MutatePodWithOpaBundleVolume(&pod, securityConfig)).To(Succeed())
			Expect(pod.Spec.Volumes).To(ContainElement(
				pods.GetOpaBundleVolume(
					securityConfig.Status.OpaBundleSource.ConfigMapName,
				),
			))
		})
	})

	Describe("MutateOpaOnPod", func() {
		It("adds the OPA init container, URL env var, and bundle volume", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: applicationRef}},
				},
			}

			Expect(pods.MutateOpaOnPod(&pod, securityConfig)).To(Succeed())
			Expect(pod.Spec.InitContainers).To(HaveLen(1))
			Expect(pods.IsOpaContainerEqual(pods.GetOpaContainer(securityConfig), pod.Spec.InitContainers[0])).To(BeTrue())
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  config.Get().OpaUrlEnvVarName,
				Value: pods.GetOpaUrlEnvVarValue(),
			}))
			Expect(pod.Spec.Volumes).To(ContainElement(
				pods.GetOpaBundleVolume(
					securityConfig.Status.OpaBundleSource.ConfigMapName,
				),
			))
		})
	})

	Describe("ValidateOpaInitContainer", func() {
		It("returns an error when the OPA init container is missing", func() {
			securityConfig := newSecurityConfig(true)
			opaInitContainer := pods.GetOpaContainer(securityConfig)
			pod := corev1.Pod{}

			Expect(pods.ValidateOpaInitContainer(pod, opaInitContainer)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but Opa init container is missing or not correctly configured for pod %s/%s",
					pod.Namespace,
					pod.Name,
				)))
		})

		It("returns an error when the OPA init container is present but not correctly configured", func() {
			securityConfig := newSecurityConfig(true)
			opaInitContainer := pods.GetOpaContainer(securityConfig)
			incorrectlyConfiguredContainer := opaInitContainer
			incorrectlyConfiguredContainer.Image = "incorrect-image"
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{incorrectlyConfiguredContainer},
				},
			}

			Expect(pods.ValidateOpaInitContainer(pod, opaInitContainer)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but Opa init container is not correctly configured for pod %s/%s",
					pod.Namespace,
					pod.Name,
				)))
		})

		It("succeeds when the OPA init container is present and correctly configured", func() {
			securityConfig := newSecurityConfig(true)
			opaInitContainer := pods.GetOpaContainer(securityConfig)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{opaInitContainer},
				},
			}

			Expect(pods.ValidateOpaInitContainer(pod, opaInitContainer)).To(Succeed())
		})
	})

	Describe("ValidateOpaURLEnvVar", func() {
		It("returns an error when the OPA URL env var is missing", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: applicationRef}},
				},
			}

			Expect(pods.ValidateOpaURLEnvVar(pod, securityConfig)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but %s env var is either missing or not correct for pod %s/%s",
					config.Get().OpaUrlEnvVarName,
					pod.Namespace,
					pod.Name,
				)))
		})

		It("returns an error when the OPA URL env var is present but has the wrong value", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().OpaUrlEnvVarName, Value: "incorrect-value"},
							},
						},
					},
				},
			}

			Expect(pods.ValidateOpaURLEnvVar(pod, securityConfig)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but %s env var value is not correct for pod %s/%s: expected %s, got %s",
					config.Get().OpaUrlEnvVarName,
					pod.Namespace,
					pod.Name,
					pods.GetOpaUrlEnvVarValue(),
					"incorrect-value",
				)))
		})

		It("succeeds when the OPA URL env var is present and has the correct value", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: applicationRef,
							Env: []corev1.EnvVar{
								{Name: config.Get().OpaUrlEnvVarName, Value: pods.GetOpaUrlEnvVarValue()},
							},
						},
					},
				},
			}

			Expect(pods.ValidateOpaURLEnvVar(pod, securityConfig)).To(Succeed())
		})
	})

	Describe("ValidateOpaBundleVolume", func() {
		It("returns an error when the OPA bundle volume is missing", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{}

			Expect(pods.ValidateOpaBundleVolume(pod, securityConfig)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but Opa bundle volume is missing for pod %s/%s",
					pod.Namespace,
					pod.Name,
				)))
		})

		It("returns an error when the OPA bundle volume is present but not correctly configured", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: pods.OpaBundleVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "incorrect-configmap",
									},
								},
							},
						},
					},
				},
			}

			Expect(pods.ValidateOpaBundleVolume(pod, securityConfig)).
				To(MatchError(fmt.Sprintf(
					"pod is annotated to have Opa, but Opa bundle volume is not correctly configured for pod %s/%s",
					pod.Namespace,
					pod.Name,
				)))
		})

		It("succeeds when the OPA bundle volume is present and correctly configured", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						pods.GetOpaBundleVolume(
							securityConfig.Status.OpaBundleSource.ConfigMapName,
						),
					},
				},
			}

			Expect(pods.ValidateOpaBundleVolume(pod, securityConfig)).To(Succeed())
		})
	})

	Describe("ValidateOpaOnPod", func() {
		It("succeeds when OPA init container, URL env var, and bundle volume are correctly configured", func() {
			securityConfig := newSecurityConfig(true)
			pod := corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: applicationRef}},
				},
			}

			Expect(pods.MutateOpaOnPod(&pod, securityConfig)).To(Succeed())
			Expect(pods.ValidateOpaOnPod(pod, securityConfig, pods.GetOpaContainer(securityConfig))).To(Succeed())
		})
	})
})
