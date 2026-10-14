package annotations

const (
	ReloaderMatchAnnotationKey   = "reloader.stakater.com/match"
	ReloaderMatchAnnotationValue = "true"
)

// ReloaderMatchAnnotation returns the annotation applied to every ConfigMap and Secret created by Accesserator,
// so that workloads can opt in to restarting on changes via the reloader.stakater.com/search annotation.
func ReloaderMatchAnnotation() map[string]string {
	return map[string]string{
		ReloaderMatchAnnotationKey: ReloaderMatchAnnotationValue,
	}
}
