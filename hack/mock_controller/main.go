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

type mockMaskinportenReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *mockMaskinportenReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("name", req.Name, "namespace", req.Namespace)

	_, maskinportenErr := reconcileMaskinporten(r, ctx, req, logger)
	if maskinportenErr != nil {
		return reconcile.Result{}, maskinportenErr
	}

	return reconcile.Result{}, nil
}

type mockAzureAdReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *mockAzureAdReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("name", req.Name, "namespace", req.Namespace)

	_, azureAdErr := reconcileAzureAdApplication(r, ctx, req, logger)
	if azureAdErr != nil {
		return reconcile.Result{}, azureAdErr
	}

	return reconcile.Result{}, nil
}

func reconcileMaskinporten(
	r *mockMaskinportenReconciler,
	ctx context.Context,
	req reconcile.Request,
	logger logr.Logger,
) (reconcile.Result, error) {
	var maskinportenClient naisiov1.MaskinportenClient
	if err := r.client.Get(ctx, req.NamespacedName, &maskinportenClient); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	logger.Info(fmt.Sprintf(
		"Reconciling MaskinportenClient %s/%s",
		maskinportenClient.GetNamespace(),
		maskinportenClient.GetName(),
	))
	secretKey := types.NamespacedName{
		Namespace: maskinportenClient.GetNamespace(),
		Name:      maskinportenClient.Spec.SecretName,
	}

	var existing corev1.Secret
	if err := r.client.Get(ctx, secretKey, &existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		logger.Info(
			fmt.Sprintf("Secret %s/%s does not exist. Creating it...", secretKey.Namespace, secretKey.Name))
		newSecret := corev1.Secret{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      maskinportenClient.Spec.SecretName,
				Namespace: maskinportenClient.GetNamespace(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: getMaskinportenSecretData(),
		}

		if setControllerRefErr := controllerutil.SetControllerReference(
			&maskinportenClient,
			&newSecret,
			r.scheme,
		); setControllerRefErr != nil {
			return reconcile.Result{}, setControllerRefErr
		}

		if createErr := r.client.Create(ctx, &newSecret); createErr != nil {
			return reconcile.Result{}, createErr
		}
		logger.Info(fmt.Sprintf("Successfully created Secret %s/%s.", secretKey.Namespace, secretKey.Name))
		maskinportenClient.Status.SynchronizationState = MaskinportenClientSynchronizationStateReady
		maskinportenClient.Status.SynchronizationSecretName = secretKey.Name
		logger.Info(
			fmt.Sprintf(
				"Updating status for MaskinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		statusErr := setMaskinportenClientStatus(ctx, r.client, maskinportenClient)
		if statusErr != nil {
			logger.Error(
				statusErr,
				fmt.Sprintf(
					"Failed to update status for MaskinportenClient %s/%s.",
					maskinportenClient.GetNamespace(),
					maskinportenClient.GetName(),
				),
			)
			return reconcile.Result{}, statusErr
		}
		logger.Info(
			fmt.Sprintf(
				"Successfully updated status for MaskinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		logger.Info(
			fmt.Sprintf(
				"Reconciliation complete for MaskinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		return reconcile.Result{}, nil
	}

	logger.Info(
		fmt.Sprintf(
			"Secret %s/%s already exists. Checking if it needs to be updated...",
			secretKey.Namespace,
			secretKey.Name,
		),
	)

	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}

	needsUpdate := false
	for key, val := range getMaskinportenSecretData() {
		if string(existing.Data[key]) != string(val) {
			existing.Data[key] = val
			needsUpdate = true
		}
	}
	if needsUpdate {
		logger.Info(
			fmt.Sprintf("Secret %s/%s needs update. Updating it...", secretKey.Namespace, secretKey.Name))
		if err := r.client.Update(ctx, &existing); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to update Secret %s/%s", secretKey.Namespace, secretKey.Name))
			return reconcile.Result{}, err
		}
	} else {
		logger.Info(
			fmt.Sprintf("Secret %s/%s is up to date. No update needed.", secretKey.Namespace, secretKey.Name))
	}
	maskinportenClient.Status.SynchronizationState = MaskinportenClientSynchronizationStateReady
	maskinportenClient.Status.SynchronizationSecretName = secretKey.Name
	logger.Info(
		fmt.Sprintf(
			"Updating status for MaskinportenClient %s/%s.",
			maskinportenClient.GetNamespace(),
			maskinportenClient.GetName(),
		),
	)
	statusErr := setMaskinportenClientStatus(ctx, r.client, maskinportenClient)
	if statusErr != nil {
		logger.Error(
			statusErr,
			fmt.Sprintf(
				"Failed to update status for MaskinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		return reconcile.Result{}, statusErr
	}
	logger.Info(
		fmt.Sprintf(
			"Successfully updated status for MaskinportenClient %s/%s.",
			maskinportenClient.GetNamespace(),
			maskinportenClient.GetName(),
		),
	)
	logger.Info(
		fmt.Sprintf(
			"Reconciliation complete for MaskinportenClient %s/%s.",
			maskinportenClient.GetNamespace(),
			maskinportenClient.GetName(),
		),
	)
	return reconcile.Result{}, nil
}

func reconcileAzureAdApplication(
	r *mockAzureAdReconciler,
	ctx context.Context,
	req reconcile.Request,
	logger logr.Logger,
) (reconcile.Result, error) {
	var azureAdApplication naisiov1.AzureAdApplication
	if err := r.client.Get(ctx, req.NamespacedName, &azureAdApplication); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	logger.Info(fmt.Sprintf(
		"Reconciling AzureAdApplication %s/%s",
		azureAdApplication.GetNamespace(),
		azureAdApplication.GetName(),
	))
	secretKey := types.NamespacedName{
		Namespace: azureAdApplication.GetNamespace(),
		Name:      azureAdApplication.Spec.SecretName,
	}

	var existing corev1.Secret
	if err := r.client.Get(ctx, secretKey, &existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		logger.Info(
			fmt.Sprintf("Secret %s/%s does not exist. Creating it...", secretKey.Namespace, secretKey.Name))
		newSecret := corev1.Secret{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      azureAdApplication.Spec.SecretName,
				Namespace: azureAdApplication.GetNamespace(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: getEntraIdSecretData(),
		}

		if setControllerRefErr := controllerutil.SetControllerReference(
			&azureAdApplication,
			&newSecret,
			r.scheme,
		); setControllerRefErr != nil {
			return reconcile.Result{}, setControllerRefErr
		}

		if createErr := r.client.Create(ctx, &newSecret); createErr != nil {
			return reconcile.Result{}, createErr
		}
		logger.Info(fmt.Sprintf("Successfully created Secret %s/%s.", secretKey.Namespace, secretKey.Name))
		azureAdApplication.Status.SynchronizationState = AzureAdApplicationSynchronizationStateReady
		azureAdApplication.Status.SynchronizationSecretName = secretKey.Name
		logger.Info(
			fmt.Sprintf(
				"Updating status for MaskinportenClient %s/%s.",
				azureAdApplication.GetNamespace(),
				azureAdApplication.GetName(),
			),
		)
		statusErr := setAzureAdApplicationStatus(ctx, r.client, azureAdApplication)
		if statusErr != nil {
			logger.Error(
				statusErr,
				fmt.Sprintf(
					"Failed to update status for AzureAdApplication %s/%s.",
					azureAdApplication.GetNamespace(),
					azureAdApplication.GetName(),
				),
			)
			return reconcile.Result{}, statusErr
		}
		logger.Info(
			fmt.Sprintf(
				"Successfully updated status for AzureAdApplication %s/%s.",
				azureAdApplication.GetNamespace(),
				azureAdApplication.GetName(),
			),
		)
		logger.Info(
			fmt.Sprintf(
				"Reconciliation complete for AzureAdApplication %s/%s.",
				azureAdApplication.GetNamespace(),
				azureAdApplication.GetName(),
			),
		)
		return reconcile.Result{}, nil
	}

	logger.Info(
		fmt.Sprintf(
			"Secret %s/%s already exists. Checking if it needs to be updated...",
			secretKey.Namespace,
			secretKey.Name,
		),
	)

	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}

	needsUpdate := false
	for key, val := range getEntraIdSecretData() {
		if string(existing.Data[key]) != string(val) {
			existing.Data[key] = val
			needsUpdate = true
		}
	}
	if needsUpdate {
		logger.Info(
			fmt.Sprintf("Secret %s/%s needs update. Updating it...", secretKey.Namespace, secretKey.Name))
		if err := r.client.Update(ctx, &existing); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to update Secret %s/%s", secretKey.Namespace, secretKey.Name))
			return reconcile.Result{}, err
		}
	} else {
		logger.Info(
			fmt.Sprintf("Secret %s/%s is up to date. No update needed.", secretKey.Namespace, secretKey.Name))
	}
	azureAdApplication.Status.SynchronizationState = AzureAdApplicationSynchronizationStateReady
	azureAdApplication.Status.SynchronizationSecretName = secretKey.Name
	logger.Info(
		fmt.Sprintf(
			"Updating status for MaskinportenClient %s/%s.",
			azureAdApplication.GetNamespace(),
			azureAdApplication.GetName(),
		),
	)
	statusErr := setAzureAdApplicationStatus(ctx, r.client, azureAdApplication)
	if statusErr != nil {
		logger.Error(
			statusErr,
			fmt.Sprintf(
				"Failed to update status for AzureAdApplication %s/%s.",
				azureAdApplication.GetNamespace(),
				azureAdApplication.GetName(),
			),
		)
		return reconcile.Result{}, statusErr
	}
	logger.Info(
		fmt.Sprintf(
			"Successfully updated status for AzureAdApplication %s/%s.",
			azureAdApplication.GetNamespace(),
			azureAdApplication.GetName(),
		),
	)
	logger.Info(
		fmt.Sprintf(
			"Reconciliation complete for AzureAdApplication %s/%s.",
			azureAdApplication.GetNamespace(),
			azureAdApplication.GetName(),
		),
	)
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

	maskinportenReconciler := &mockMaskinportenReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
	if createMaskinportenControllerErr := ctrl.NewControllerManagedBy(mgr).
		For(&naisiov1.MaskinportenClient{}).
		Owns(&corev1.Secret{}).
		Complete(maskinportenReconciler); createMaskinportenControllerErr != nil {
		fmt.Fprintf(os.Stderr, "failed to set up controller: %v\n", createMaskinportenControllerErr)
		os.Exit(1)
	}

	azureAdReconciler := &mockAzureAdReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
	if createEntraIdControllerErr := ctrl.NewControllerManagedBy(mgr).
		For(&naisiov1.AzureAdApplication{}).
		Owns(&corev1.Secret{}).
		Complete(azureAdReconciler); createEntraIdControllerErr != nil {
		fmt.Fprintf(os.Stderr, "failed to set up controller: %v\n", createEntraIdControllerErr)
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
