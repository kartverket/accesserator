package eventhandler_test

import (
	"context"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	skiperatorv1alpha1 "github.com/kartverket/skiperator/api/v1alpha1"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func buildClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(accesseratorv1alpha.AddToScheme(scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(skiperatorv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(naisiov1.AddToScheme(scheme)).To(Succeed())

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func runCreateEvent(h handler.EventHandler, obj client.Object) []reconcile.Request {
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	h.Create(context.Background(), event.CreateEvent{Object: obj}, queue)

	requests := make([]reconcile.Request, 0, queue.Len())
	for queue.Len() > 0 {
		item, _ := queue.Get()
		requests = append(requests, item)
		queue.Done(item)
	}

	return requests
}

func req(namespace, name string) reconcile.Request {
	return reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}
}
