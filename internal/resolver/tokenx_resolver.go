package resolver

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveTokenXConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.TokenXConfig, error) {
	tokenXEnabled := securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled
	if !tokenXEnabled {
		return &state.TokenXConfig{
			Enabled: tokenXEnabled,
		}, nil
	}

	var skiperatorApplication v1alpha1.Application
	if exists := k8sClient.Get(ctx, types.NamespacedName{
		Name:      string(securityConfig.Spec.ApplicationRef),
		Namespace: securityConfig.Namespace,
	}, &skiperatorApplication); exists != nil {
		return nil, fmt.Errorf(
			"failed to fetch Application resource named %s: %w",
			securityConfig.Spec.ApplicationRef,
			exists,
		)
	}

	jwkerInboundRules, err := resolveJwkerInboundRules(ctx, k8sClient, securityConfig, skiperatorApplication)
	if err != nil {
		return nil, err
	}

	naisIoV1AccessPolicyInboundRules := naisiov1.AccessPolicyInboundRules{}
	if jwkerInboundRules != nil {
		naisIoV1AccessPolicyInboundRules = jwkerInboundRules
	}

	return &state.TokenXConfig{
		Enabled:        tokenXEnabled,
		ApplicationRef: string(securityConfig.Spec.ApplicationRef),
		JwkerSpec: naisiov1.JwkerSpec{
			SecretName: utilities.NewTokenxNamer(securityConfig).SecretName(),
			AccessPolicy: &naisiov1.AccessPolicy{
				Inbound: &naisiov1.AccessPolicyInbound{
					Rules: naisIoV1AccessPolicyInboundRules,
				},
				// Jwker outbound access policy is required, but not relevant for token exchange.
				Outbound: &naisiov1.AccessPolicyOutbound{},
			},
		},
	}, nil
}

func resolveJwkerInboundRules(
	ctx context.Context,
	k8sClient client.Client,
	securityConfig v1alpha.SecurityConfig,
	skiperatorApplication v1alpha1.Application,
) ([]naisiov1.AccessPolicyInboundRule, error) {
	var jwkerInboundRules []naisiov1.AccessPolicyInboundRule

	if securityConfig.Spec.Tokenx.AccessPolicy == nil {
		return jwkerInboundRules, nil
	}

	if securityConfig.Spec.Tokenx.AccessPolicy.InheritInboundRules &&
		skiperatorApplication.Spec.AccessPolicy != nil &&
		skiperatorApplication.Spec.AccessPolicy.Inbound != nil &&
		len(skiperatorApplication.Spec.AccessPolicy.Inbound.Rules) > 0 {
		for _, inboundAccessPolicyRule := range skiperatorApplication.Spec.AccessPolicy.Inbound.Rules {
			namespaceList, errs := getNamespaceListForInboundRule(
				ctx,
				inboundAccessPolicyRule,
				securityConfig.Namespace,
				k8sClient,
			)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to resolve Namespaces: %w", errors.Join(errs...))
			}
			for _, namespace := range namespaceList {
				jwkerInboundRules = append(jwkerInboundRules, naisiov1.AccessPolicyInboundRule{
					AccessPolicyRule: naisiov1.AccessPolicyRule{
						Application: inboundAccessPolicyRule.Application,
						Namespace:   namespace,
						Cluster:     config.Get().ClusterName,
					},
				})
			}
		}
	}

	for _, accessPolicyClient := range securityConfig.Spec.Tokenx.AccessPolicy.Clients {
		namespace := securityConfig.Namespace
		if accessPolicyClient.Namespace != nil {
			namespace = string(*accessPolicyClient.Namespace)
		}

		jwkerInboundRules = append(jwkerInboundRules, naisiov1.AccessPolicyInboundRule{
			AccessPolicyRule: naisiov1.AccessPolicyRule{
				Application: string(accessPolicyClient.Application),
				Namespace:   namespace,
				Cluster:     config.Get().ClusterName,
			},
		})
	}

	return utilities.UniqueSliceElements(jwkerInboundRules), nil
}

/*
getNamespaceListForInboundRule resolves a list of namespaces given an InternalRule:
  - If Namespace is set, only this single namespace is returned
  - If NamespacesByLabel is set, the corresponding namespaces are returned
  - Otherwise, the SecurityConfig's namespace is returned, which is the same as the Skiperator Application
*/
func getNamespaceListForInboundRule(
	ctx context.Context,
	rule podtypes.InternalRule,
	securityConfigNamespace string,
	k8sClient client.Client,
) ([]string, []error) {
	var ruleErrors []error
	var namespaceList []string
	switch {
	case rule.Namespace != "":
		namespaceList = append(namespaceList, rule.Namespace)
	case len(rule.NamespacesByLabel) != 0:
		namespaces, err := getNamespacesByLabel(ctx, rule, k8sClient)
		if err != nil {
			ruleErrors = append(ruleErrors, err)
		}
		for _, ns := range namespaces.Items {
			namespaceList = append(namespaceList, ns.Name)
		}
	default:
		namespaceList = append(namespaceList, securityConfigNamespace)
	}

	slices.Sort(namespaceList)
	return namespaceList, ruleErrors
}

/*
getNamespacesByLabel takes an InternalRule inbound access policy with a non-nil NamespacesByLabel value, and fetches the
corresponding namespaces using the Kubernetes API.
*/
func getNamespacesByLabel(
	ctx context.Context,
	rule podtypes.InternalRule,
	k8sClient client.Client,
) (*corev1.NamespaceList, error) {
	namespaces := &corev1.NamespaceList{}
	selector := metav1.LabelSelector{MatchLabels: rule.NamespacesByLabel}
	selectorString, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return namespaces, fmt.Errorf("failed to create label selector: %w", err)
	}
	if err = k8sClient.List(ctx, namespaces, &client.ListOptions{LabelSelector: selectorString}); err != nil {
		return namespaces, fmt.Errorf("failed to list namespaces: %w", err)
	}
	return namespaces, nil
}
