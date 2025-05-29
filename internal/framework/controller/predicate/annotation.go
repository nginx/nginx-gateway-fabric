package predicate

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
)

// AnnotationPredicate implements a predicate function based on the Annotation.
//
// This predicate will skip the following events:
// 1. Create events that do not contain the Annotation.
// 2. Update events where the Annotation value has not changed.
type AnnotationPredicate struct {
	predicate.Funcs
	Annotation string
}

// Create filters CreateEvents based on the Annotation.
func (ap AnnotationPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	_, ok := e.Object.GetAnnotations()[ap.Annotation]
	return ok
}

// Update filters UpdateEvents based on the Annotation.
func (ap AnnotationPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		// this case should not happen
		return false
	}

	oldAnnotationVal := e.ObjectOld.GetAnnotations()[ap.Annotation]
	newAnnotationVal := e.ObjectNew.GetAnnotations()[ap.Annotation]

	return oldAnnotationVal != newAnnotationVal
}

// RestartDeploymentAnnotationPredicate skips update events if they are due to a rolling restart.
// This type of event is triggered by adding an annotation to the deployment's PodSpec.
// This is used by the provisioner to ensure it allows for rolling restarts of the nginx deployment
// without reverting the annotation and deleting the new pod(s). Otherwise, if a user changes
// the nginx deployment, we want to see that event so we can revert it back to the configuration
// that we expect it to have.
type RestartDeploymentAnnotationPredicate struct {
	predicate.Funcs
}

// Update filters UpdateEvents based on if the annotation is present or changed.
func (RestartDeploymentAnnotationPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		// this case should not happen
		return false
	}

	depOld, ok := e.ObjectOld.(*appsv1.Deployment)
	if !ok {
		return false
	}

	depNew, ok := e.ObjectNew.(*appsv1.Deployment)
	if !ok {
		return false
	}

	oldVal, oldExists := depOld.Spec.Template.Annotations[controller.RestartedAnnotation]

	if newVal, ok := depNew.Spec.Template.Annotations[controller.RestartedAnnotation]; ok {
		if !oldExists || newVal != oldVal {
			return false
		}
	}

	return true
}
