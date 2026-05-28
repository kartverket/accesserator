package statusmanager_test

import (
	"context"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockControllerResource implements the reconciliation.ControllerResource interface.
type mockControllerResource struct {
	resourceKind string
	resourceName string
	isNil        bool
}

func (m *mockControllerResource) GetResourceKind() string { return m.resourceKind }
func (m *mockControllerResource) GetResourceName() string { return m.resourceName }
func (m *mockControllerResource) IsResourceNil() bool     { return m.isNil }
func (m *mockControllerResource) Reconcile(
	_ context.Context,
	_ client.Client,
	_ *runtime.Scheme,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func newMockResource(kind, name string, isNil bool) reconciliation.ControllerResource {
	return &mockControllerResource{
		resourceKind: kind,
		resourceName: name,
		isNil:        isNil,
	}
}

// nolint:unparam
func newTestJwker(namespace, appName, synchronizationState, secretName string) *naisiov1.Jwker {
	return &naisiov1.Jwker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utilities.TokenxNamer{ApplicationRef: appName}.JwkerName(),
			Namespace: namespace,
		},
		Status: naisiov1.JwkerStatus{
			SynchronizationState:      synchronizationState,
			SynchronizationSecretName: secretName,
		},
	}
}

// nolint:unparam
func newTestMaskinportenClient(namespace, appName, synchronizationState, secretName string) *naisiov1.MaskinportenClient {
	return &naisiov1.MaskinportenClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utilities.MaskinportenNamer{ApplicationRef: appName}.MaskinportenClientName(),
			Namespace: namespace,
		},
		Status: naisiov1.DigdiratorStatus{
			SynchronizationState:      synchronizationState,
			SynchronizationSecretName: secretName,
		},
	}
}

func newK8sClientWithObjects(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = accesseratorv1alpha.AddToScheme(scheme)
	_ = naisiov1.AddToScheme(scheme)
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objects {
		builder = builder.WithObjects(obj)
	}
	return builder.Build()
}
