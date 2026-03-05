# Accesserator

Accesserator is a Kubernetes operator that lets users configure security capabilities and 
make them available for [Skiperator](https://github.com/kartverket/skiperator) applications through a custom resource called `SecurityConfig`. 
To make the security capabilities easily available for the pod created by the Skiperator application, accesserator injects a sidecar container called 
[texas](https://github.com/nais/texas) into the application pod. Texas is a sidecar container that provides an API for applications to request different token-related operations, 
such as retrieval of access tokens, token exchange and token introspection. The API-documentation for the texas sidecar container can be found in [here](https://editor.swagger.io/?url=https://raw.githubusercontent.com/nais/texas/refs/heads/master/doc/openapi-spec.json).

The injection of `texas` is controlled through a pod annotation called `accesserator.kartverket.no/services`, which can be set in the Skiperator `Application` 
through the field `spec.podSettings.annotations`. If the annotation is set to `accesserator.kartverket.no/services: texas`, 
accesserator will inject the `texas` sidecar into the application pod and configure it according to the `SecurityConfig` that references the application. 
The accepted fields of `SecurityConfig`  can be found in the [API reference](api-docs.md) below.

> [!TIP]
> If you do not want the `texas` sidecar, but you still want accesserator to verify that you have configured a `SercurityConfig` for your application, 
> you can set the annotation `accesserator.kartverket.no/verify-securityconfig: true`. This will make accesserator verify that you have ONE `SecurityConfig` for your application, 
> and deny the pod creation if this is not the case.

## 🔧 Examples
In the [examples](examples/) directory, you can find examples on how to configure different security capabilities with accesserator.

## 🧪 Local development

Refer to [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to run and test accesserator locally.