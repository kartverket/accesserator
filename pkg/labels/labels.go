package labels

const (
	ManagedByLabelKey  = "app.kubernetes.io/managed-by"
	ControllerLabelKey = "accesserator.kartverket.no/controller"

	ManagedByLabelValue                = "accesserator"
	SecurityConfigControllerLabelValue = "securityconfig"
)

// SecurityConfigStandardLabels returns the set of labels applied to every resource created by Accesserator for the
// SecurityConfig controller.
func SecurityConfigStandardLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:  ManagedByLabelValue,
		ControllerLabelKey: SecurityConfigControllerLabelValue,
	}
}
