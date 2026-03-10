package statusmanager_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/internal/statusmanager"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const forceConditionChangeMessage = "Different message to force condition change"
const createdSuccessfullyMessage = "Created successfully"

func newTestSecurityConfig() *v1alpha.SecurityConfig {
	return &v1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-security-config",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityConfig",
			APIVersion: "accesserator.kartverket.no/v1alpha",
		},
	}
}

var _ = Describe("BuildSecurityConfigCondition", func() {
	var sc *v1alpha.SecurityConfig

	BeforeEach(func() {
		sc = newTestSecurityConfig()
	})

	It("returns a False condition with error message when state is Invalid", func() {
		validationError := "some validation error"
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateInvalid, &validationError, []metav1.Condition{})

		Expect(condition.Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("InvalidConfiguration"))
		Expect(condition.Message).To(Equal(validationError))
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns an Unknown condition when state is Pending", func() {
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StatePending, utilities.Ptr("ignored"), []metav1.Condition{})

		Expect(condition.Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(condition.Status).To(Equal(metav1.ConditionUnknown))
		Expect(condition.Reason).To(Equal("ReconciliationPending"))
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns an Unknown condition when state is WaitingForJwker", func() {
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateWaitingForJwker, utilities.Ptr("ignored"), []metav1.Condition{})

		Expect(condition.Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(condition.Status).To(Equal(metav1.ConditionUnknown))
		Expect(condition.Reason).To(Equal("ReconciliationPending"))
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns a False condition when state is Failed", func() {
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateFailed, utilities.Ptr("ignored"), []metav1.Condition{})

		Expect(condition.Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("ReconciliationFailed"))
		Expect(condition.Message).To(Equal("Descendants of SecurityConfig failed during reconciliation."))
	})

	It("returns a True condition when state is Ready", func() {
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{})

		Expect(condition.Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		Expect(condition.Reason).To(Equal("ReconciliationSuccess"))
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("sets a new LastTransitionTime when no existing conditions", func() {
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{})
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse(), "LastTransitionTime should be set")
	})

	It("preserves LastTransitionTime when condition is identical to existing", func() {
		existing := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{})
		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{existing})

		Expect(condition.LastTransitionTime).To(Equal(existing.LastTransitionTime),
			"LastTransitionTime should be preserved when condition is unchanged")
	})

	It("updates LastTransitionTime when condition has changed", func() {
		existing := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{})
		existing.Message = forceConditionChangeMessage

		condition := statusmanager.BuildSecurityConfigCondition(sc, statusmanager.StateReady, utilities.Ptr("ignored"), []metav1.Condition{existing})

		Expect(condition.LastTransitionTime).NotTo(Equal(existing.LastTransitionTime),
			"LastTransitionTime should be updated when condition changes")
		Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
	})
})

var _ = Describe("BuildDescendantConditions", func() {
	It("returns an empty slice when there are no descendants", func() {
		conditions := statusmanager.BuildDescendantConditions([]state.Descendant[client.Object]{}, []metav1.Condition{})
		Expect(conditions).To(BeEmpty())
	})

	It("returns a True condition for a successful descendant", func() {
		successMsg := "Resource created successfully"
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Type).To(Equal("Secret-my-secret"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[0].Reason).To(Equal("Success"))
		Expect(conditions[0].Message).To(Equal(successMsg))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns a False condition for a failed descendant", func() {
		errorMsg := "Failed to create resource"
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}, ErrorMessage: &errorMsg},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Type).To(Equal("Secret-my-secret"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal("Error"))
		Expect(conditions[0].Message).To(Equal(errorMsg))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns an Unknown condition for a descendant without a message", func() {
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Type).To(Equal("Secret-my-secret"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionUnknown))
		Expect(conditions[0].Reason).To(Equal("Unknown"))
		Expect(conditions[0].Message).To(Equal("No status message set"))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("returns all conditions for multiple descendants", func() {
		successMsg := "Created"
		errorMsg := "Failed"
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-secret-1", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
			{ID: "Secret-secret-2", Object: &corev1.Secret{}, ErrorMessage: &errorMsg},
			{ID: "ConfigMap-config-1", Object: &corev1.ConfigMap{}},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

		Expect(conditions).To(HaveLen(3))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[1].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[2].Status).To(Equal(metav1.ConditionUnknown))
	})

	It("sets a new LastTransitionTime when no existing conditions", func() {
		successMsg := createdSuccessfullyMessage
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse(), "LastTransitionTime should be set")
	})

	It("preserves LastTransitionTime when condition is identical to existing", func() {
		successMsg := createdSuccessfullyMessage
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
		}
		existingConditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})
		Expect(existingConditions).To(HaveLen(1))

		conditions := statusmanager.BuildDescendantConditions(descendants, existingConditions)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).To(Equal(existingConditions[0].LastTransitionTime),
			"LastTransitionTime should be preserved when condition is unchanged")
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("updates LastTransitionTime when condition has changed", func() {
		successMsg := createdSuccessfullyMessage
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
		}
		existingConditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})
		Expect(existingConditions).To(HaveLen(1))
		existingConditions[0].Message = forceConditionChangeMessage

		conditions := statusmanager.BuildDescendantConditions(descendants, existingConditions)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).NotTo(Equal(existingConditions[0].LastTransitionTime),
			"LastTransitionTime should be updated when condition changes")
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})
})

var _ = Describe("BuildMissingResourceConditions", func() {
	It("returns an empty slice when all resources are present", func() {
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-my-secret", Object: &corev1.Secret{}},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "my-secret", false),
		}

		conditions := statusmanager.BuildMissingResourceConditions(descendants, resources, []metav1.Condition{})
		Expect(conditions).To(BeEmpty())
	})

	It("returns a False condition for a missing resource", func() {
		descendants := []state.Descendant[client.Object]{}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "expected-secret", false),
		}

		conditions := statusmanager.BuildMissingResourceConditions(descendants, resources, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Type).To(Equal("Secret-expected-secret"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal("NotFound"))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("ignores nil resources", func() {
		descendants := []state.Descendant[client.Object]{}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "some-secret", true),
		}

		conditions := statusmanager.BuildMissingResourceConditions(descendants, resources, []metav1.Condition{})
		Expect(conditions).To(BeEmpty(), "Nil resources should be ignored")
	})

	It("returns only missing resources when some are present", func() {
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-present-secret", Object: &corev1.Secret{}},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "present-secret", false),
			newMockResource("ConfigMap", "missing-config", false),
			newMockResource("Secret", "missing-secret", false),
		}

		conditions := statusmanager.BuildMissingResourceConditions(descendants, resources, []metav1.Condition{})

		Expect(conditions).To(HaveLen(2))
		Expect(conditions[0].Type).To(Equal("ConfigMap-missing-config"))
		Expect(conditions[1].Type).To(Equal("Secret-missing-secret"))
	})

	It("sets a new LastTransitionTime when no existing conditions", func() {
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "expected-secret", false),
		}

		conditions := statusmanager.BuildMissingResourceConditions(nil, resources, []metav1.Condition{})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse(), "LastTransitionTime should be set")
	})

	It("preserves LastTransitionTime when condition is identical to existing", func() {
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "expected-secret", false),
		}
		existingConditions := statusmanager.BuildMissingResourceConditions(nil, resources, []metav1.Condition{})
		Expect(existingConditions).To(HaveLen(1))

		conditions := statusmanager.BuildMissingResourceConditions(nil, resources, existingConditions)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).To(Equal(existingConditions[0].LastTransitionTime),
			"LastTransitionTime should be preserved when condition is unchanged")
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("updates LastTransitionTime when condition has changed", func() {
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "expected-secret", false),
		}
		existingConditions := statusmanager.BuildMissingResourceConditions(nil, resources, []metav1.Condition{})
		Expect(existingConditions).To(HaveLen(1))
		existingConditions[0].Message = forceConditionChangeMessage

		conditions := statusmanager.BuildMissingResourceConditions(nil, resources, existingConditions)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).NotTo(Equal(existingConditions[0].LastTransitionTime),
			"LastTransitionTime should be updated when condition changes")
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})
})

var _ = Describe("BuildConditions", func() {
	It("includes SecurityConfig condition, descendant conditions, and missing resource conditions", func() {
		sc := newTestSecurityConfig()
		successMsg := createdSuccessfullyMessage
		descendants := []state.Descendant[client.Object]{
			{ID: "Secret-oauth-secret", Object: &corev1.Secret{}, SuccessMessage: &successMsg},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "oauth-secret", false),
			newMockResource("NetworkPolicy", "my-netpol", false),
		}

		conditions := statusmanager.BuildConditions(sc, statusmanager.StateReady, utilities.Ptr("ignored"), descendants, resources, []metav1.Condition{})

		Expect(conditions).To(HaveLen(3), "Should have SecurityConfig + descendant + missing resource condition")
		Expect(conditions[0].Type).To(Equal("SecurityConfig-test-security-config"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[1].Type).To(Equal("Secret-oauth-secret"))
		Expect(conditions[1].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[2].Type).To(Equal("NetworkPolicy-my-netpol"))
		Expect(conditions[2].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[2].Reason).To(Equal("NotFound"))
	})
})
