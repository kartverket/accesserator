package pods

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func EnsureUnusedContainerName(podToMutate *corev1.Pod, sidecarContainer corev1.Container) error {
	for _, container := range append(podToMutate.Spec.InitContainers, podToMutate.Spec.Containers...) {
		if container.Name == sidecarContainer.Name {
			return fmt.Errorf("pod already has a container named %s", sidecarContainer.Name)
		}
	}
	return nil
}

func EnsureUnusedContainerPorts(podToMutate *corev1.Pod, sidecarContainer corev1.Container) error {
	for _, container := range append(podToMutate.Spec.InitContainers, podToMutate.Spec.Containers...) {
		for _, containerPort := range container.Ports {
			for _, sidecarContainerPort := range sidecarContainer.Ports {
				if containerPort.ContainerPort == sidecarContainerPort.ContainerPort {
					return fmt.Errorf("pod already has a port on %d", sidecarContainerPort.ContainerPort)
				}
			}
		}
	}
	return nil
}

func EnsureUnusedEnvVar(container corev1.Container, envVarName string) error {
	for _, env := range container.Env {
		if env.Name == envVarName {
			return fmt.Errorf("container %s already has env var %s", container.Name, envVarName)
		}
	}
	return nil
}
