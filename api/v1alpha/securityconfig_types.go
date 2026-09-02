/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha

import (
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityConfigSpec defines the desired state of SecurityConfig.
type SecurityConfigSpec struct {
	// Tokenx specifies whether to configure the token exchange capability for an application referred to by `applicationRef`.
	// accessPolicies of the application referred to by applicationRef
	// will be used to restrict which applications can exchange tokens where the specified application is the intended audience.
	//
	// +kubebuilder:validation:Optional
	Tokenx *TokenXSpec `json:"tokenx,omitempty"`

	// Maskinporten specifies whether to configure the maskinporten API consumer capability for an application referred to by `applicationRef`.
	// The configuration can either be provided inline via the `client` field,
	// by referencing an existing MaskinportenClient resource via the `clientRef` field,
	// or by sourcing credentials from existing Kubernetes secrets via the `secretRef` field.
	//
	// +kubebuilder:validation:Optional
	Maskinporten *MaskinportenSpec `json:"maskinporten,omitempty"`

	// EntraID specifies whether to configure the Entra ID API consumer capability for an application referred to by `applicationRef`.
	// The configuration can either be provided inline via the `client` field,
	// by referencing an existing AzureAdApplication resource via the `clientRef` field,
	// or by sourcing credentials from existing Kubernetes secrets via the `secretRef` field.
	//
	// +kubebuilder:validation:Optional
	EntraID *EntraIDSpec `json:"entraid,omitempty"`

	// Idporten specifies whether to configure ID-porten token validation for an application referred to by `applicationRef`.
	// When enabled, an Istio ServiceEntry is created to allow egress to ID-porten, and the Texas sidecar is configured
	// to validate ID-porten tokens against the audience specified in `allowedAudience`.
	//
	// +kubebuilder:validation:Optional
	Idporten *IdPortenSpec `json:"idporten,omitempty"`

	// Ansattporten specifies whether to configure Ansattporten token validation for an application referred to by `applicationRef`.
	// When enabled, an Istio ServiceEntry is created to allow egress to Ansattporten, and the Texas sidecar is configured
	// to validate Ansattporten tokens against the audiences specified in `allowedAudience`.
	//
	// +kubebuilder:validation:Optional
	Ansattporten *AnsattportenSpec `json:"ansattporten,omitempty"`

	// Opa specifies whether to configure the open policy agent capability for an application referred to by `applicationRef`.
	// The configuration includes which bundles compiled from rego policies, and how often OPA should check for updates to these bundles.
	//
	// +kubebuilder:validation:Optional
	Opa *OpenPolicyAgentSpec `json:"opa,omitempty"`

	// ApplicationRef is a reference to the name of the SKIP application for which this SecurityConfig applies.
	//
	// +kubebuilder:validation:Required
	ApplicationRef ResourceName `json:"applicationRef"`
}

// TokenXSpec defines the configuration for token exchange.
//
// +kubebuilder:object:generate=true
type TokenXSpec struct {
	// Enabled indicates whether token exchange should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// AccessPolicy specifies configuration of which clients can exchange tokens with the application as target when
	// token exchange is enabled. If not specified, no clients are allowed.
	//
	// +kubebuilder:validation:Optional
	AccessPolicy *AccessPolicySpec `json:"accessPolicy,omitempty"`
}

// AccessPolicySpec specifies configuration of which clients can exchange tokens with the application as target when
// token exchange is enabled. If not specified, no clients are allowed.
//
// +kubebuilder:object:generate=true
type AccessPolicySpec struct {
	// InheritInboundRules specifies whether the inbound access policy rules of the corresponding Skiperator Application
	// should be used as clients for token exchange. Defaults to false. When set to true, the complete list of clients
	// will be the union of the explicitly specified clients in Clients and the clients resolved from inbound rules.
	//
	// +kubebuilder:validation:Optional
	InheritInboundRules bool `json:"inheritInboundRules"`

	// Clients which may perform token exchange with your application as target.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:Optional
	Clients []AccessPolicyClient `json:"clients,omitempty"`
}

// AccessPolicyClient define client applications which may perform token exchange with your application as target.
//
// +kubebuilder:object:generate=true
type AccessPolicyClient struct {
	// Application is the name of the client application that can exchange tokens with the target application.
	//
	// +kubebuilder:validation:Required
	Application ResourceName `json:"application"`

	// Namespace is the namespace which the client application resides. If not specified, the namespace of the
	// SecurityConfig's referenced application will be used.
	//
	// +kubebuilder:validation:Optional
	Namespace *ResourceName `json:"namespace,omitempty"`
}

// MaskinportenSpec defines the configuration for Maskinporten.
//
// At most one of `client`, `clientRef`, or `secretRef` may be specified.
// Exactly one must be specified when `enabled` is true.
//
// +kubebuilder:object:generate=true
// +kubebuilder:validation:XValidation:rule="[has(self.client), has(self.clientRef), has(self.secretRef)].filter(x, x).size() <= 1",message="At most one of client, clientRef, or secretRef may be specified."
type MaskinportenSpec struct {
	// Enabled indicates whether Maskinporten should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Client defines the Maskinporten client configuration inline.
	// Use this when you want to configure the client directly.
	//
	// +kubebuilder:validation:Optional
	Client *MaskinportenClientSpec `json:"client,omitempty"`

	// ClientRef references an existing MaskinportenClient by name.
	// Use this when a client registration resource exists, and you want to reference it.
	//
	// +kubebuilder:validation:Optional
	ClientRef *ResourceRef `json:"clientRef,omitempty"`

	// SecretRef sources the client registration client credentials from one or more existing Kubernetes secrets.
	// Use this when you have an existing OAuth client registered outside the SecurityConfig CRD
	// and MaskinportenClient CRD (e.g. manually registered at DigDir).
	//
	// +kubebuilder:validation:Optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`
}

func (spec MaskinportenSpec) GetClient() *MaskinportenClientSpec {
	return spec.Client
}
func (spec MaskinportenSpec) GetClientRef() *ResourceRef {
	return spec.ClientRef
}
func (spec MaskinportenSpec) GetSecretRef() *SecretRef {
	return spec.SecretRef
}

// MaskinportenClientSpec defines the inline configuration for a [MaskinportenClient](https://github.com/nais/digdirator?tab=readme-ov-file#digdirator).
//
// +kubebuilder:object:generate=true
type MaskinportenClientSpec struct {
	// ClientName is the client name to be registered at DigDir.
	// It is shown during login for user-centric flows, and is otherwise a human-readable way to differentiate between clients at DigDir's self-service portal.
	//
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Required
	ClientName string `json:"clientName"`

	// Scopes is an object of consumed scopes.
	//
	// +kubebuilder:validation:Optional
	Scopes *MaskinportenScope `json:"scopes,omitempty"`
}

// MaskinportenScope defines consumed scopes for the application.
//
// +kubebuilder:object:generate=true
type MaskinportenScope struct {
	// `consumes` is a list of scopes that your client can request access to.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:Required
	ConsumedScopes []naisiov1.ConsumedScope `json:"consumes"`
}

// EntraIDSpec defines the configuration for Entra ID.
//
// At most one of `client`, `clientRef`, or `secretRef` may be specified.
// Exactly one must be specified when `enabled` is true.
//
// +kubebuilder:object:generate=true
// +kubebuilder:validation:XValidation:rule="[has(self.client), has(self.clientRef), has(self.secretRef)].filter(x, x).size() <= 1",message="At most one of client, clientRef, or secretRef may be specified."
type EntraIDSpec struct {
	// Enabled indicates whether Entra ID should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Client defines the Entra ID client configuration inline.
	// Use this when you want to configure the client directly.
	//
	// +kubebuilder:validation:Optional
	Client *AzureAdApplicationSpec `json:"client,omitempty"`

	// ClientRef references an existing AzureAdApplication by name.
	// Use this when a AzureAdApplication exists, and you want to reference it.
	//
	// +kubebuilder:validation:Optional
	ClientRef *ResourceRef `json:"clientRef,omitempty"`

	// SecretRef sources the client registration credentials from one or more existing Kubernetes secrets.
	// Use this when you have an existing OAuth client registered outside the SecurityConfig CRD
	// and AzureAdApplication CRD (e.g. manually registered at Entra).
	//
	// +kubebuilder:validation:Optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`
}

func (spec EntraIDSpec) GetClient() *AzureAdApplicationSpec {
	return spec.Client
}
func (spec EntraIDSpec) GetClientRef() *ResourceRef {
	return spec.ClientRef
}
func (spec EntraIDSpec) GetSecretRef() *SecretRef {
	return spec.SecretRef
}

// AzureAdApplicationSpec defines the inline configuration for a [AzureAdApplication](https://github.com/nais/azurerator?tab=readme-ov-file#azurerator).
//
// +kubebuilder:object:generate=true
type AzureAdApplicationSpec struct {
	// SecretName is the name of the resulting Secret resource to be created. If not set, the secret will be given a
	// a name based on the name of the SecurityConfig resource.
	//
	// +kubebuilder:validation:Optional
	SecretName string `json:"secretName,omitempty"`

	// Groups is a list of Entra ID group IDs to be emitted in the `groups` claim in tokens issued by Entra ID. This
	// also assigns groups to the application for access control. Only direct members of the groups are granted access.
	//
	// +kubebuilder:validation:Optional
	Groups []naisiov1.AzureAdGroup `json:"groups,omitempty"`

	// LogoutUrl is the URL where Entra ID sends a request to have the application clear the user's session data. This
	// is required if single sign-out should work correctly. Must start with 'https'
	//
	// +kubebuilder:validation:Optional
	LogoutUrl string `json:"logoutUrl,omitempty"`

	// PreAuthorizedApplications is a list of Entra ID Applications that are authorized to perform client credential
	// flow with this application as scope, or the on-behalf-of (OBO) flow.
	//
	// +kubebuilder:validation:Optional
	PreAuthorizedApplications []naisiov1.AccessPolicyInboundRule `json:"preAuthorizedApplications,omitempty"`

	// ReplyUrls is a list of authorized redirect URIs Entra ID may use when performing authorization code flow. All
	// production URLs must use the 'https' scheme.
	//
	// +kubebuilder:validation:Optional
	ReplyUrls []naisiov1.AzureAdReplyUrl `json:"replyUrls,omitempty"`

	// SinglePageApplication denotes whether this Entra ID application should be registered as a single-page-application
	// for usage in client-side applications without access to secrets.
	//
	// +kubebuilder:validation:Optional
	SinglePageApplication *bool `json:"singlePageApplication,omitempty"`
}

// IdPortenSpec defines the configuration for ID-porten token validation.
//
// +kubebuilder:object:generate=true
type IdPortenSpec struct {
	// Enabled indicates whether ID-porten token validation should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// AllowedAudience defines the audience (`aud`) value that ID-porten tokens are validated against by the Texas
	// sidecar. Either a static value or sourced from a ConfigMap or Secret.
	//
	// +kubebuilder:validation:Required
	AllowedAudience AllowedAudience `json:"allowedAudience"`
}

// AnsattportenSpec defines the configuration for Ansattporten token validation.
//
// +kubebuilder:object:generate=true
type AnsattportenSpec struct {
	// Enabled indicates whether Ansattporten token validation should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// AllowedAudience defines the audience (`aud`) value that Ansattporten tokens are validated against by the Texas
	// sidecar. Either a static value or sourced from a ConfigMap or Secret.
	//
	// +kubebuilder:validation:Required
	AllowedAudience AllowedAudience `json:"allowedAudience"`
}

// OpenPolicyAgentSpec defines the OPA sidecar configuration.
//
// +kubebuilder:object:generate=true
type OpenPolicyAgentSpec struct {
	// Enabled indicates whether OPA should be configured for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// BundleURLs is a list of URLs pointing to OPA bundles containing compiled rego policies.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=20
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.name == x.name).size() == 1)",message="Each bundle name must be unique"
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.url == x.url).size() == 1)",message="Each bundle URL must be unique"
	BundleURLs []BundleSource `json:"bundleUrls"`

	// RequestPolicy enables per-request evaluation against the OPA sidecar via Envoy's external authorization
	// (ext_authz) filter. When configured, every incoming request to the application is sent to OPA for
	// policy evaluation before it reaches the application container. The rego policy decides the outcome,
	// which may be authorization (allow/deny) or request enrichment (e.g. adding headers).
	//
	// +kubebuilder:validation:Optional
	RequestPolicy *OpaRequestPolicy `json:"requestPolicy,omitempty"`
}

// OpaRequestPolicy configures the Envoy ext_authz integration that consults the OPA sidecar on each
// incoming request. The referenced rego rule at `Endpoint` is evaluated for every request and its
// decision determines whether the request is forwarded, denied, or mutated.
//
// +kubebuilder:object:generate=true
type OpaRequestPolicy struct {
	// Enabled indicates whether Envoy should call the OPA sidecar for external authorization on each
	// request. When false, no EnvoyFilter is installed and requests bypass OPA evaluation entirely.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Endpoint is the OPA Data API endpoint that Envoy queries for each request. It must start with
	// `/v1/data` and be followed by the slash-separated package/rule path of the rego rule to evaluate
	// (e.g. `/v1/data/envoy/authz/allow`). The referenced rule must be defined in one of the loaded
	// bundles.
	//
	// +kubebuilder:validation:Pattern=`^/v1/data(/[a-z_][a-z0-9_]*)+$`
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// FailureMode determines how Envoy handles requests when the OPA sidecar is unreachable or returns
	// an error (e.g. HTTP 5xx). `DENY` fails closed and rejects the request, which is the
	// safe choice when OPA is used for authorization. `FORWARD` fails open and lets the request through
	// to the application; only use this when OPA is used purely for enrichment or non-critical checks,
	// never for authorization decisions.
	// Defaults to DENY.
	//
	// +kubebuilder:validation:Enum=FORWARD;DENY
	// +kubebuilder:validation:Optional
	FailureMode string `json:"failureMode,omitempty"`
}

// BundleSource defines a source for an OPA bundle.
//
// +kubebuilder:object:generate=true
type BundleSource struct {
	// Name specifies a human-readable name for the OPA bundle.
	// It is used to differentiate between bundles which is used by OPA.
	//
	// +kubebuilder:validation:Required
	Name DataKey `json:"name"`

	// URL is the OCI registry location of the OPA bundle.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=253
	URL string `json:"url"`

	// Verification specifies how to verify the integrity of the bundle.
	// If not specified, the bundle will not be verified.
	//
	// +kubebuilder:validation:Optional
	Verification *BundleSourceVerification `json:"verification,omitempty"`
}

// BundleSourceVerification specifies how to verify the integrity of the bundle.
//
// +kubebuilder:object:generate=true
type BundleSourceVerification struct {
	// Source is the GitHub repository where the bundle was created.
	//
	// +kubebuilder:validation:Required
	Source GitHubRepositorySource `json:"source"`
}

type GitHubRepositorySource struct {
	// Repository is the GitHub repository where the bundle was created,
	// in the form "<org-or-user>/<repo>" (e.g. "kartverket/accesserator").
	//
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}/[a-zA-Z0-9_][a-zA-Z0-9._-]{0,99}$`
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=140
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// Workflow is the name of the GitHub workflow that created the bundle.
	//
	// +kubebuilder:validation:Pattern=`^\.github/workflows/[a-zA-Z0-9_.-]+\.(yml|yaml)$`
	// +kubebuilder:validation:MaxLength=300
	// +kubebuilder:validation:Optional
	Workflow string `json:"workflow,omitempty"`

	// Ref is the Git reference (branch, tag, or commit) of the bundle.
	//
	// +kubebuilder:validation:Pattern=`^refs/(heads|tags|pull)/[a-zA-Z0-9/_.-]{1,243}$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="!self.contains('..')",message="ref must not contain '..'"
	// +kubebuilder:validation:XValidation:rule="!self.endsWith('.lock')",message="ref must not end with '.lock'"
	// +kubebuilder:validation:Optional
	Ref string `json:"ref,omitempty"`
}

// AllowedAudience defines an audience that is validated against the `aud` claim in the JWT.
// An audience can be defined as a static value or retrieved from a kubernetes resource.
//
// +kubebuilder:validation:XValidation:message="either 'value' or 'valueFrom' must be set",rule="has(self.value) || has(self.valueFrom)"
// +kubebuilder:validation:XValidation:message="one audience cannot be defined from both 'value' and 'valueFrom'",rule="!(has(self.value) && has(self.valueFrom))"
// +kubebuilder:validation:XValidation:message="field 'value' cannot be empty string",rule="!has(self.value) || size(self.value) > 0"
// +kubebuilder:object:generate=true
type AllowedAudience struct {
	// Value specifies a static audience value.
	//
	// +kubebuilder:validation:Optional
	Value *string `json:"value,omitempty"`

	// ValueFrom specifies a reference to a kubernetes resource to retrieve the audience value from.
	//
	// +kubebuilder:validation:Optional
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

// ValueFrom specifies a reference to a kubernetes resource to retrieve a value from.
//
// +kubebuilder:validation:XValidation:message="either 'configMapKeyRef' or 'secretKeyRef' must be set",rule="has(self.configMapKeyRef) || has(self.secretKeyRef)"
// +kubebuilder:validation:XValidation:message="cannot reference both a ConfigMap and a Secret",rule="!(has(self.configMapKeyRef) && has(self.secretKeyRef))"
// +kubebuilder:object:generate=true
type ValueFrom struct {
	// ConfigMapKeyRef specifies a reference to a key in a ConfigMap.
	//
	// +kubebuilder:validation:Optional
	ConfigMapKeyRef *KeyRef `json:"configMapKeyRef,omitempty"`

	// SecretKeyRef specifies a reference to a key in a Secret.
	//
	// +kubebuilder:validation:Optional
	SecretKeyRef *KeyRef `json:"secretKeyRef,omitempty"`
}

// KeyRef specifies a reference to a specific key within a kubernetes resource.
//
// +kubebuilder:object:generate=true
type KeyRef struct {
	// Name specifies the name of the ConfigMap/Secret; must satisfy DNS-1123 subdomain naming.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key specifies the data entry name within the ConfigMap/Secret; must follow key naming rules.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// ResourceRef defines a reference to an existing resource by name.
//
// +kubebuilder:object:generate=true
type ResourceRef struct {
	// Name of the referenced resource.
	//
	// +kubebuilder:validation:Required
	Name ResourceName `json:"name"`
}

// DataKey is a type for keys within Kubernetes secrets and configmaps.
//
// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([-._a-zA-Z0-9]*[a-zA-Z0-9])?$`
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:MinLength=1
type DataKey string

// ResourceName is a type for Kubernetes resource names.
//
// +kubebuilder:validation:MaxLength=253
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
type ResourceName string

// SecretRef defines where to source each required Maskinporten environment variable from.
// Each field points to a key in a Kubernetes secret, allowing the values to come from
// one or more existing secrets.
//
// +kubebuilder:object:generate=true
type SecretRef struct {
	// ClientID references the secret key containing the Maskinporten client ID (MASKINPORTEN_CLIENT_ID).
	//
	// +kubebuilder:validation:Required
	ClientID SecretKeySelector `json:"clientID"`

	// ClientJWK references the secret key containing the Maskinporten client JWK (MASKINPORTEN_CLIENT_JWK).
	//
	// +kubebuilder:validation:Required
	ClientJWK SecretKeySelector `json:"clientJWK"`
}

// SecretKeySelector identifies a key within a Kubernetes secret.
//
// +kubebuilder:object:generate=true
type SecretKeySelector struct {
	// Name is the name of the Kubernetes secret.
	//
	// +kubebuilder:validation:Required
	Name ResourceName `json:"name"`

	// Key is the key within the secret whose value should be used.
	//
	// +kubebuilder:validation:Required
	Key DataKey `json:"key"`
}

// SecurityConfigStatus defines the observed state of SecurityConfig.
type SecurityConfigStatus struct {
	ObservedGeneration     int64              `json:"observedGeneration,omitempty"`
	Conditions             []metav1.Condition `json:"conditions,omitempty"`
	Phase                  Phase              `json:"phase,omitempty"`
	Message                string             `json:"message,omitempty"`
	JwkerSecretName        string             `json:"jwkerSecretName,omitempty"`
	MaskinportenSecretName string             `json:"maskinportenSecretName,omitempty"`
	EntraIdSecretName      string             `json:"entraIdSecretName,omitempty"`
	IdportenAudience       string             `json:"idportenAudience,omitempty"`
	AnsattportenAudience   string             `json:"ansattportenAudience,omitempty"`
	OpaBundleSource        *OpaBundleSource   `json:"opaBundleSource,omitempty"`
	Ready                  bool               `json:"ready"`
}

// OpaBundleSource defines the source of OPA bundles used for policy evaluation.
type OpaBundleSource struct {
	ConfigMapName string   `json:"configMapName,omitempty"`
	BundleNames   []string `json:"bundleNames,omitempty"`
}

type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseReady   Phase = "Ready"
	PhaseFailed  Phase = "Failed"
	PhaseInvalid Phase = "Invalid"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

// SecurityConfig is the Schema for the securityconfigs API
type SecurityConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SecurityConfig
	// +required
	Spec SecurityConfigSpec `json:"spec"`

	// status defines the observed state of SecurityConfig
	// +optional
	Status SecurityConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SecurityConfigList contains a list of SecurityConfig
type SecurityConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SecurityConfig `json:"items"`
}

func (s *SecurityConfig) InitializeStatus() {
	if s.Status.Conditions == nil {
		s.Status.Conditions = []metav1.Condition{}
	}
	s.Status.ObservedGeneration = s.GetGeneration()
	s.Status.Ready = false
	s.Status.Phase = PhasePending
}

func (s *SecurityConfigStatus) SetPhaseInvalid(msg string) {
	s.Phase = PhaseInvalid
	s.Ready = false
	s.Message = msg
}

func (s *SecurityConfigStatus) SetPhasePending(msg string) {
	s.Phase = PhasePending
	s.Ready = false
	s.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseFailed(msg string) {
	s.Phase = PhaseFailed
	s.Ready = false
	s.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseReady(msg string) {
	s.Phase = PhaseReady
	s.Ready = true
	s.Message = msg
}
