# API Reference

Packages:

- [accesserator.kartverket.no/v1alpha](#accesseratorkartverketnov1alpha)

# accesserator.kartverket.no/v1alpha

Resource Types:

- [SecurityConfig](#securityconfig)




## SecurityConfig
<sup><sup>[↩ Parent](#accesseratorkartverketnov1alpha )</sup></sup>






SecurityConfig is the Schema for the securityconfigs API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>accesserator.kartverket.no/v1alpha</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>SecurityConfig</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of SecurityConfig<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of SecurityConfig<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec
<sup><sup>[↩ Parent](#securityconfig)</sup></sup>



spec defines the desired state of SecurityConfig

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>applicationRef</b></td>
        <td>string</td>
        <td>
          ApplicationRef is a reference to the name of the SKIP application for which this SecurityConfig applies.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinporten">maskinporten</a></b></td>
        <td>object</td>
        <td>
          Maskinporten specifies whether to configure the maskinporten API consumer capability for an application referred to by `applicationRef`.
The configuration can either be provided inline via the `client` field,
by referencing an existing MaskinportenClient resource via the `clientRef` field,
or by sourcing credentials from existing Kubernetes secrets via the `secretRef` field.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#securityconfigspectokenx">tokenx</a></b></td>
        <td>object</td>
        <td>
          Tokenx specifies whether to configure the token exchange capability for an application referred to by `applicationRef`.
accessPolicies of the application referred to by applicationRef
will be used to restrict which applications can exchange tokens where the specified application is the intended audience.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten
<sup><sup>[↩ Parent](#securityconfigspec)</sup></sup>



Maskinporten specifies whether to configure the maskinporten API consumer capability for an application referred to by `applicationRef`.
The configuration can either be provided inline via the `client` field,
by referencing an existing MaskinportenClient resource via the `clientRef` field,
or by sourcing credentials from existing Kubernetes secrets via the `secretRef` field.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled indicates whether Maskinporten should be configured for the application.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinportenclient">client</a></b></td>
        <td>object</td>
        <td>
          Client defines the Maskinporten client configuration inline.
Use this when you want to configure the client directly.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinportenclientref">clientRef</a></b></td>
        <td>object</td>
        <td>
          ClientRef references an existing MaskinportenClient by name.
Use this when a MaskinportenClient exists, and you want to reference it.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinportensecretref">secretRef</a></b></td>
        <td>object</td>
        <td>
          SecretRef sources the Maskinporten client credentials from one or more existing Kubernetes secrets.
Use this when you have an existing OAuth client registered outside the SecurityConfig CRD
and MaskinportenClient CRD (e.g. manually registered at DigDir).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.client
<sup><sup>[↩ Parent](#securityconfigspecmaskinporten)</sup></sup>



Client defines the Maskinporten client configuration inline.
Use this when you want to configure the client directly.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>clientName</b></td>
        <td>string</td>
        <td>
          ClientName is the client name to be registered at DigDir.
It is shown during login for user-centric flows, and is otherwise a human-readable way to differentiate between clients at DigDir's self-service portal.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinportenclientscopes">scopes</a></b></td>
        <td>object</td>
        <td>
          Scopes is an object of consumed scopes.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.client.scopes
<sup><sup>[↩ Parent](#securityconfigspecmaskinportenclient)</sup></sup>



Scopes is an object of consumed scopes.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#securityconfigspecmaskinportenclientscopesconsumesindex">consumes</a></b></td>
        <td>[]object</td>
        <td>
          `consumes` is a list of scopes that your client can request access to.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.client.scopes.consumes[index]
<sup><sup>[↩ Parent](#securityconfigspecmaskinportenclientscopes)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The scope consumed by the application to gain access to an external organization API.
Ensure that the NAV organization has been granted access to the scope prior to requesting access.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.clientRef
<sup><sup>[↩ Parent](#securityconfigspecmaskinporten)</sup></sup>



ClientRef references an existing MaskinportenClient by name.
Use this when a MaskinportenClient exists, and you want to reference it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referenced MaskinportenClient.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.secretRef
<sup><sup>[↩ Parent](#securityconfigspecmaskinporten)</sup></sup>



SecretRef sources the Maskinporten client credentials from one or more existing Kubernetes secrets.
Use this when you have an existing OAuth client registered outside the SecurityConfig CRD
and MaskinportenClient CRD (e.g. manually registered at DigDir).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#securityconfigspecmaskinportensecretrefclientid">clientID</a></b></td>
        <td>object</td>
        <td>
          ClientID references the secret key containing the Maskinporten client ID (MASKINPORTEN_CLIENT_ID).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspecmaskinportensecretrefclientjwk">clientJWK</a></b></td>
        <td>object</td>
        <td>
          ClientJWK references the secret key containing the Maskinporten client JWK (MASKINPORTEN_CLIENT_JWK).<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.secretRef.clientID
<sup><sup>[↩ Parent](#securityconfigspecmaskinportensecretref)</sup></sup>



ClientID references the secret key containing the Maskinporten client ID (MASKINPORTEN_CLIENT_ID).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the key within the secret whose value should be used.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the Kubernetes secret.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.maskinporten.secretRef.clientJWK
<sup><sup>[↩ Parent](#securityconfigspecmaskinportensecretref)</sup></sup>



ClientJWK references the secret key containing the Maskinporten client JWK (MASKINPORTEN_CLIENT_JWK).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the key within the secret whose value should be used.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the Kubernetes secret.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.tokenx
<sup><sup>[↩ Parent](#securityconfigspec)</sup></sup>



Tokenx specifies whether to configure the token exchange capability for an application referred to by `applicationRef`.
accessPolicies of the application referred to by applicationRef
will be used to restrict which applications can exchange tokens where the specified application is the intended audience.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled indicates whether token exchange should be configured for the application.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.status
<sup><sup>[↩ Parent](#securityconfig)</sup></sup>



status defines the observed state of SecurityConfig

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>ready</b></td>
        <td>boolean</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>jwkerSecretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>maskinportenSecretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>phase</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.status.conditions[index]
<sup><sup>[↩ Parent](#securityconfigstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>