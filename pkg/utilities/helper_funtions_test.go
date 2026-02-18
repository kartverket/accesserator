package utilities_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestPtr(t *testing.T) {
	v := 42
	ptr := utilities.Ptr(v)
	assert.NotNil(t, ptr)
	assert.Equal(t, v, *ptr)
}

func TestLowestNonZeroResult(t *testing.T) {
	zero := ctrl.Result{}
	one := ctrl.Result{RequeueAfter: 1 * time.Second}
	two := ctrl.Result{RequeueAfter: 2 * time.Second}

	assert.Equal(t, zero, utilities.LowestNonZeroResult(zero, zero))
	assert.Equal(t, one, utilities.LowestNonZeroResult(zero, one))
	assert.Equal(t, one, utilities.LowestNonZeroResult(one, zero))
	assert.Equal(t, one, utilities.LowestNonZeroResult(one, two))
	assert.Equal(t, one, utilities.LowestNonZeroResult(two, one))
}

func TestGetJwkerName(t *testing.T) {
	appRef := "my-app"
	assert.Equal(t, appRef, utilities.GetJwkerName(appRef))
}

func TestGetJwkerSecretName(t *testing.T) {
	jwkerName := "foo"
	want := fmt.Sprintf("%s-%s", jwkerName, utilities.JwkerSecretNameSuffix)
	assert.Equal(t, want, utilities.GetJwkerSecretName(jwkerName))
}

func TestGetTokenxEgressName(t *testing.T) {
	secName := "sec"
	tokenx := "tok"
	want := fmt.Sprintf("%s-%s-%s", secName, tokenx, utilities.EgressNameSuffix)
	assert.Equal(t, want, utilities.GetTokenxEgressName(secName, tokenx))
}

func TestGetMockKubernetesClient(t *testing.T) {
	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	client := utilities.GetMockKubernetesClient(scheme, obj)
	assert.NotNil(t, client)
}
