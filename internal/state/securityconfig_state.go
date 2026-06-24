package state

import (
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	InlineClient ConfigType = iota
	ClientRef
	SecretRef
	None
)

type ConfigType int

type Scope struct {
	SecurityConfig         v1alpha.SecurityConfig
	TokenXConfig           TokenXConfig
	MaskinportenConfig     MaskinportenConfig
	EntraIdConfig          EntraIdConfig
	IdPortenConfig         IdPortenConfig
	AnsattportenConfig     AnsattportenConfig
	OpaConfig              OpaConfig
	Descendants            []Descendant[client.Object]
	InvalidConfig          bool
	ValidationErrorMessage *string
}

type TokenXConfig struct {
	Enabled        bool
	ApplicationRef string
	JwkerSpec      naisiov1.JwkerSpec
}

type MaskinportenConfig struct {
	Enabled    bool
	Type       ConfigType
	ClientSpec *naisiov1.MaskinportenClientSpec
	ClientRef  *v1alpha.ResourceRef
	SecretData *map[string][]byte
}

type EntraIdConfig struct {
	Enabled    bool
	Type       ConfigType
	ClientSpec *naisiov1.AzureAdApplicationSpec
	ClientRef  *v1alpha.ResourceRef
	SecretData *map[string][]byte
}

type IdPortenConfig struct {
	Enabled  bool
	Audience string
}

type AnsattportenConfig struct {
	Enabled  bool
	Audience string
}

type OpaConfig struct {
	Enabled          bool
	BundleBinaryData map[string][]byte
}

type Descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

func (s *Scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.Descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *Scope) ReplaceDescendant(
	obj client.Object,
	errorMessage *string,
	successMessage *string,
	resourceKind, resourceName string,
) {
	if s != nil {
		expectedID := GetID(resourceKind, resourceName)
		for i, d := range s.Descendants {
			if d.ID == expectedID {
				s.Descendants[i] = Descendant[client.Object]{
					ID:             expectedID,
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.Descendants = append(s.Descendants, Descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}

func (s *Scope) IsMisconfigured() bool {
	return s.InvalidConfig
}
