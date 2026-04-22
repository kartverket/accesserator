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
// +kubebuilder:validation:XValidation:rule="[has(self.client), has(self.clientRef), has(self.secretRef)].filter(x, x).size() <= 1",message="At most one of client, clientRef, or secretRef may be specified"
// +kubebuilder:validation:XValidation:rule="!self.enabled || [has(self.client), has(self.clientRef), has(self.secretRef)].filter(x, x).size() == 1",message="Exactly one of client, clientRef, or secretRef must be specified when enabled is true"
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
	// Use this when a MaskinportenClient exists, and you want to reference it.
	//
	// +kubebuilder:validation:Optional
	ClientRef *MaskinportenClientRef `json:"clientRef,omitempty"`

	// SecretRef sources the Maskinporten client credentials from one or more existing Kubernetes secrets.
	// Use this when you have an existing OAuth client registered outside the SecurityConfig CRD
	// and MaskinportenClient CRD (e.g. manually registered at DigDir).
	//
	// +kubebuilder:validation:Optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`
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

// MaskinportenClientRef defines a reference to an existing MaskinportenClient by name.
//
// +kubebuilder:object:generate=true
type MaskinportenClientRef struct {
	// Name of the referenced MaskinportenClient.
	//
	// +kubebuilder:validation:Required
	Name ResourceName `json:"name"`
}

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
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([-._a-zA-Z0-9]*[a-zA-Z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// SecurityConfigStatus defines the observed state of SecurityConfig.
type SecurityConfigStatus struct {
	ObservedGeneration      int64              `json:"observedGeneration,omitempty"`
	Conditions              []metav1.Condition `json:"conditions,omitempty"`
	Phase                   Phase              `json:"phase,omitempty"`
	Message                 string             `json:"message,omitempty"`
	JwkerSecretName         string             `json:"jwkerSecretName,omitempty"`
	MaskinportenSectretName string             `json:"maskinportenSecretName,omitempty"`
	Ready                   bool               `json:"ready"`
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

func init() {
	SchemeBuilder.Register(&SecurityConfig{}, &SecurityConfigList{})
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

func SetConditionInvalid(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionFalse
	cond.Reason = "InvalidConfiguration"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhasePending(msg string) {
	s.Phase = PhasePending
	s.Ready = false
	s.Message = msg
}

func SetConditionPending(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionUnknown
	cond.Reason = "ReconciliationPending"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseFailed(msg string) {
	s.Phase = PhaseFailed
	s.Ready = false
	s.Message = msg
}

func SetConditionFailed(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionFalse
	cond.Reason = "ReconciliationFailed"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseReady(msg string) {
	s.Phase = PhaseReady
	s.Ready = true
	s.Message = msg
}

func SetConditionReady(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionTrue
	cond.Reason = "ReconciliationSuccess"
	cond.Message = msg
}
