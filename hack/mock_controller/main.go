package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kartverket/accesserator/pkg/utilities"
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

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(naisiov1.AddToScheme(scheme))
}

type mockReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *mockReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("name", req.Name, "namespace", req.Namespace)

	var maskinportenClient naisiov1.MaskinportenClient
	if err := r.client.Get(ctx, req.NamespacedName, &maskinportenClient); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	logger.Info(fmt.Sprintf(
		"Reconciling MaksinportenClient %s/%s",
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
		logger.Info(fmt.Sprintf("Secret %s/%s does not exist. Creating it...", secretKey.Namespace, secretKey.Name))
		newSecret := corev1.Secret{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      maskinportenClient.Spec.SecretName,
				Namespace: maskinportenClient.GetNamespace(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: getSecretData(),
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
		maskinportenClient.Status.SynchronizationState = utilities.SynchronizationStateReady
		maskinportenClient.Status.SynchronizationSecretName = secretKey.Name
		logger.Info(
			fmt.Sprintf(
				"Updating status for MaksinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		statusErr := setMaskinportenClientStatus(ctx, r.client, maskinportenClient)
		if statusErr != nil {
			logger.Error(
				statusErr,
				fmt.Sprintf(
					"Failed to update status for MaksinportenClient %s/%s.",
					maskinportenClient.GetNamespace(),
					maskinportenClient.GetName(),
				),
			)
			return reconcile.Result{}, statusErr
		}
		logger.Info(
			fmt.Sprintf(
				"Successfully updated status for MaksinportenClient %s/%s.",
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
	for key, val := range getSecretData() {
		if string(existing.Data[key]) != string(val) {
			existing.Data[key] = val
			needsUpdate = true
		}
	}
	if needsUpdate {
		logger.Info(fmt.Sprintf("Secret %s/%s needs update. Updating it...", secretKey.Namespace, secretKey.Name))
		if err := r.client.Update(ctx, &existing); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to update Secret %s/%s", secretKey.Namespace, secretKey.Name))
			return reconcile.Result{}, err
		}
	} else {
		logger.Info(fmt.Sprintf("Secret %s/%s is up to date. No update needed.", secretKey.Namespace, secretKey.Name))
	}
	maskinportenClient.Status.SynchronizationState = utilities.SynchronizationStateReady
	maskinportenClient.Status.SynchronizationSecretName = secretKey.Name
	logger.Info(
		fmt.Sprintf(
			"Updating status for MaksinportenClient %s/%s.",
			maskinportenClient.GetNamespace(),
			maskinportenClient.GetName(),
		),
	)
	statusErr := setMaskinportenClientStatus(ctx, r.client, maskinportenClient)
	if statusErr != nil {
		logger.Error(
			statusErr,
			fmt.Sprintf(
				"Failed to update status for MaksinportenClient %s/%s.",
				maskinportenClient.GetNamespace(),
				maskinportenClient.GetName(),
			),
		)
		return reconcile.Result{}, statusErr
	}
	logger.Info(
		fmt.Sprintf(
			"Successfully updated status for MaksinportenClient %s/%s.",
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

	reconciler := &mockReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}

	if createControllerErr := ctrl.NewControllerManagedBy(mgr).
		For(&naisiov1.MaskinportenClient{}).
		Owns(&corev1.Secret{}).
		Complete(reconciler); createControllerErr != nil {
		fmt.Fprintf(os.Stderr, "failed to set up controller: %v\n", createControllerErr)
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

func getSecretData() map[string][]byte {
	return map[string][]byte{
		"MASKINPORTEN_ISSUER":         []byte("https://test.maskinporten.no/"),
		"MASKINPORTEN_TOKEN_ENDPOINT": []byte("https://test.maskinporten.no/token"),
		"MASKINPORTEN_JWKS_URI":       []byte("https://test.maskinporten.no/jwk"),
		"MASKINPORTEN_CLIENT_ID":      []byte(getEnv("MASKINPORTEN_CLIENT_ID")),
		"MASKINPORTEN_CLIENT_JWK":     []byte(getEnv("MASKINPORTEN_CLIENT_JWK")),
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

func getEnv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	panic(fmt.Sprintf("environment variable %s not set", key))
}
