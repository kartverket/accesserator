package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var scheme = runtime.NewScheme()

const (
	MaskinportenClientSynchronizationStateReady = "Synchronized"
	AzureAdApplicationSynchronizationStateReady = "Synchronized"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(naisiov1.AddToScheme(scheme))
}

// reconcileConfig captures the type-specific behaviour for a single mock reconciler instance.
type reconcileConfig struct {
	// resourceKind is the human-readable kind name used in log messages.
	resourceKind string
	// newObject returns a zero-value instance of the owned resource type.
	newObject func() client.Object
	// getSecretName extracts the desired secret name from the owned resource.
	getSecretName func(client.Object) string
	// getSecretData returns the desired secret key/value pairs.
	getSecretData func() map[string][]byte
	// setStatus persists the synchronization status back onto the owned resource.
	setStatus func(ctx context.Context, k8sClient client.Client, obj client.Object, secretName string) error
}

type mockReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config reconcileConfig
}

func (r *mockReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("name", req.Name, "namespace", req.Namespace)

	obj := r.config.newObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	logger.Info(fmt.Sprintf(
		"Reconciling %s %s/%s",
		r.config.resourceKind,
		obj.GetNamespace(),
		obj.GetName(),
	))

	return reconcileSecret(
		ctx, r.client, r.scheme, logger,
		obj,
		r.config.resourceKind,
		r.config.getSecretName(obj),
		r.config.getSecretData(),
		func(secretName string) error {
			return r.config.setStatus(ctx, r.client, obj, secretName)
		},
	)
}

// reconcileSecret contains the shared logic for creating or updating a Secret owned by a
// resource, followed by a status update. The caller provides the variable parts:
//   - owner: the Kubernetes object that owns the secret
//   - resourceKind: human-readable kind name used in log messages
//   - secretName: name of the secret to manage
//   - secretData: desired secret key/value pairs
//   - setStatus: sets the owner's status fields and persists them; receives the secret name
func reconcileSecret(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	logger logr.Logger,
	owner client.Object,
	resourceKind string,
	secretName string,
	secretData map[string][]byte,
	setStatus func(secretName string) error,
) (reconcile.Result, error) {
	secretKey := types.NamespacedName{
		Namespace: owner.GetNamespace(),
		Name:      secretName,
	}

	var existing corev1.Secret
	if err := k8sClient.Get(ctx, secretKey, &existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		logger.Info(fmt.Sprintf("Secret %s/%s does not exist. Creating it...", secretKey.Namespace, secretKey.Name))
		newSecret := corev1.Secret{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      secretName,
				Namespace: owner.GetNamespace(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretData,
		}
		if err := controllerutil.SetControllerReference(owner, &newSecret, scheme); err != nil {
			return reconcile.Result{}, err
		}
		if err := k8sClient.Create(ctx, &newSecret); err != nil {
			return reconcile.Result{}, err
		}
		logger.Info(fmt.Sprintf("Successfully created Secret %s/%s.", secretKey.Namespace, secretKey.Name))
	} else {
		logger.Info(fmt.Sprintf(
			"Secret %s/%s already exists. Checking if it needs to be updated...",
			secretKey.Namespace,
			secretKey.Name,
		))

		if existing.Data == nil {
			existing.Data = map[string][]byte{}
		}
		needsUpdate := false
		for key, val := range secretData {
			if string(existing.Data[key]) != string(val) {
				existing.Data[key] = val
				needsUpdate = true
			}
		}
		if needsUpdate {
			logger.Info(fmt.Sprintf("Secret %s/%s needs update. Updating it...", secretKey.Namespace, secretKey.Name))
			if err := k8sClient.Update(ctx, &existing); err != nil {
				logger.Error(err, fmt.Sprintf("Failed to update Secret %s/%s", secretKey.Namespace, secretKey.Name))
				return reconcile.Result{}, err
			}
		} else {
			logger.Info(fmt.Sprintf("Secret %s/%s is up to date. No update needed.", secretKey.Namespace, secretKey.Name))
		}
	}

	logger.Info(fmt.Sprintf(
		"Updating status for %s %s/%s.",
		resourceKind,
		owner.GetNamespace(),
		owner.GetName(),
	))
	if err := setStatus(secretKey.Name); err != nil {
		logger.Error(err, fmt.Sprintf(
			"Failed to update status for %s %s/%s.",
			resourceKind,
			owner.GetNamespace(),
			owner.GetName(),
		))
		return reconcile.Result{}, err
	}
	logger.Info(fmt.Sprintf(
		"Successfully updated status for %s %s/%s.",
		resourceKind,
		owner.GetNamespace(),
		owner.GetName(),
	))
	logger.Info(fmt.Sprintf(
		"Reconciliation complete for %s %s/%s.",
		resourceKind,
		owner.GetNamespace(),
		owner.GetName(),
	))
	return reconcile.Result{}, nil
}

func main() {
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))

	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("Starting mock controller")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: ":8083",
		Metrics:                server.Options{BindAddress: "0"},
		LeaderElection:         false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create manager: %v\n", err)
		os.Exit(1)
	}

	maskinportenReconciler := &mockReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: reconcileConfig{
			resourceKind: "MaskinportenClient",
			newObject:    func() client.Object { return &naisiov1.MaskinportenClient{} },
			getSecretName: func(obj client.Object) string {
				return obj.(*naisiov1.MaskinportenClient).Spec.SecretName
			},
			getSecretData: getMaskinportenSecretData,
			setStatus: func(ctx context.Context, k8sClient client.Client, obj client.Object, secretName string) error {
				mc := obj.(*naisiov1.MaskinportenClient)
				mc.Status.SynchronizationState = MaskinportenClientSynchronizationStateReady
				mc.Status.SynchronizationSecretName = secretName
				return setMaskinportenClientStatus(ctx, k8sClient, *mc)
			},
		},
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&naisiov1.MaskinportenClient{}).
		Owns(&corev1.Secret{}).
		Complete(maskinportenReconciler); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set up controller: %v\n", err)
		os.Exit(1)
	}

	azureAdReconciler := &mockReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: reconcileConfig{
			resourceKind: "AzureAdApplication",
			newObject:    func() client.Object { return &naisiov1.AzureAdApplication{} },
			getSecretName: func(obj client.Object) string {
				return obj.(*naisiov1.AzureAdApplication).Spec.SecretName
			},
			getSecretData: getEntraIdSecretData,
			setStatus: func(ctx context.Context, k8sClient client.Client, obj client.Object, secretName string) error {
				app := obj.(*naisiov1.AzureAdApplication)
				app.Status.SynchronizationState = AzureAdApplicationSynchronizationStateReady
				app.Status.SynchronizationSecretName = secretName
				return setAzureAdApplicationStatus(ctx, k8sClient, *app)
			},
		},
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&naisiov1.AzureAdApplication{}).
		Owns(&corev1.Secret{}).
		Complete(azureAdReconciler); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set up controller: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add healthz check: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add readyz check: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager exited with error")
		os.Exit(1)
	}
}

func getMaskinportenSecretData() map[string][]byte {
	return map[string][]byte{
		"MASKINPORTEN_ISSUER":         []byte("https://test.maskinporten.no/"),
		"MASKINPORTEN_TOKEN_ENDPOINT": []byte("https://test.maskinporten.no/token"),
		"MASKINPORTEN_JWKS_URI":       []byte("https://test.maskinporten.no/jwk"),
		"MASKINPORTEN_CLIENT_ID":      []byte(getEnv("MASKINPORTEN_CLIENT_ID")),
		"MASKINPORTEN_CLIENT_JWK":     []byte(getEnv("MASKINPORTEN_CLIENT_JWK")),
	}
}

func getEntraIdSecretData() map[string][]byte {
	return map[string][]byte{
		"AZURE_OPENID_CONFIG_ISSUER": []byte(
			"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0"),
		"AZURE_OPENID_CONFIG_TOKEN_ENDPOINT": []byte(
			"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/token"),
		"AZURE_OPENID_CONFIG_JWKS_URI": []byte(
			"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/discovery/v2.0/keys"),
		"AZURE_APP_CLIENT_ID": []byte(getEnv("AZURE_APP_CLIENT_ID")),
		"AZURE_APP_JWK":       []byte(getEnv("AZURE_APP_JWK")),
	}
}

func setMaskinportenClientStatus(
	ctx context.Context,
	k8sClient client.Client,
	maskinportenClient naisiov1.MaskinportenClient,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &naisiov1.MaskinportenClient{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&maskinportenClient), latest); err != nil {
			return err
		}
		latest.Status = maskinportenClient.Status
		return k8sClient.Status().Update(ctx, latest)
	})
}

func setAzureAdApplicationStatus(
	ctx context.Context,
	k8sClient client.Client,
	azureadapplication naisiov1.AzureAdApplication,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &naisiov1.AzureAdApplication{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&azureadapplication), latest); err != nil {
			return err
		}
		latest.Status = azureadapplication.Status
		return k8sClient.Status().Update(ctx, latest)
	})
}

func getEnv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	panic(fmt.Sprintf("environment variable %s not set", key))
}
