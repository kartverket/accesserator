package validation_test

import (
	"context"
	"strings"
	"testing"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/validation"
	skiperatorv1alpha1 "github.com/kartverket/skiperator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateApplicationRef_NoSkiperatorApplication_ReturnsError(t *testing.T) {
	applicationRef := "test-app"
	notApplicationref := applicationRef + "-not"

	app := &skiperatorv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: notApplicationref,
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(app).Build()

	scope := &state.Scope{
		SecurityConfig: accesseratorv1alpha.SecurityConfig{
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				ApplicationRef: "test-app",
			},
		},
	}

	err := validation.ValidateApplicationRef(context.Background(), k8sClient, scope)
	if err == nil {
		t.Errorf("Expected error when Skiperator app doesn't exist, got none")
	} else if !strings.Contains(err.Error(), applicationRef) {
		t.Errorf("Expected error to mention '%s', got '%s'", applicationRef, err.Error())
	}
}

func TestValidateApplicationRef_SkiperatorApplicationExists_ReturnsNil(t *testing.T) {
	applicationRef := "test-app"

	app := &skiperatorv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationRef,
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(app).Build()

	scope := &state.Scope{
		SecurityConfig: accesseratorv1alpha.SecurityConfig{
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				ApplicationRef: accesseratorv1alpha.ResourceName(applicationRef),
			},
		},
	}

	err := validation.ValidateApplicationRef(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("Expected nil error when Skiperator app exist, got none")
	}
}

func scheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = skiperatorv1alpha1.AddToScheme(s)
	return s
}
