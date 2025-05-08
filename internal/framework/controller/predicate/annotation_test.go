package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
)

func TestAnnotationPredicate_Create(t *testing.T) {
	t.Parallel()
	annotation := "test"

	tests := []struct {
		event     event.CreateEvent
		name      string
		expUpdate bool
	}{
		{
			name: "object has annotation",
			event: event.CreateEvent{
				Object: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "object does not have annotation",
			event: event.CreateEvent{
				Object: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"diff": "one",
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name:      "object does not have any annotations",
			event:     event.CreateEvent{Object: &apiext.CustomResourceDefinition{}},
			expUpdate: false,
		},
		{
			name:      "object is nil",
			event:     event.CreateEvent{Object: nil},
			expUpdate: false,
		},
	}

	p := AnnotationPredicate{Annotation: annotation}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			update := p.Create(test.event)
			g.Expect(update).To(Equal(test.expUpdate))
		})
	}
}

func TestAnnotationPredicate_Update(t *testing.T) {
	t.Parallel()
	annotation := "test"

	tests := []struct {
		event     event.UpdateEvent
		name      string
		expUpdate bool
	}{
		{
			name: "annotation changed",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
				ObjectNew: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "two",
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "annotation deleted",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
				ObjectNew: &apiext.CustomResourceDefinition{},
			},
			expUpdate: true,
		},
		{
			name: "annotation added",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{},
				ObjectNew: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "annotation has not changed",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
				ObjectNew: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "different annotation changed",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"diff": "one",
						},
					},
				},
				ObjectNew: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"diff": "two",
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "no annotations",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{},
				ObjectNew: &apiext.CustomResourceDefinition{},
			},
			expUpdate: false,
		},
		{
			name: "old object is nil",
			event: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "new object is nil",
			event: event.UpdateEvent{
				ObjectOld: &apiext.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotation: "one",
						},
					},
				},
				ObjectNew: nil,
			},
			expUpdate: false,
		},
		{
			name: "both objects are nil",
			event: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: nil,
			},
			expUpdate: false,
		},
	}

	p := AnnotationPredicate{Annotation: annotation}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			update := p.Update(test.event)
			g.Expect(update).To(Equal(test.expUpdate))
		})
	}
}

func TestRestartDeploymentAnnotationPredicate_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		event     event.UpdateEvent
		name      string
		expUpdate bool
	}{
		{
			name: "annotation added",
			event: event.UpdateEvent{
				ObjectOld: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{},
							},
						},
					},
				},
				ObjectNew: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "annotation changed",
			event: event.UpdateEvent{
				ObjectOld: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "false",
								},
							},
						},
					},
				},
				ObjectNew: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "annotation removed",
			event: event.UpdateEvent{
				ObjectOld: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
				ObjectNew: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{},
							},
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "annotation unchanged",
			event: event.UpdateEvent{
				ObjectOld: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
				ObjectNew: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "old object is nil",
			event: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "new object is nil",
			event: event.UpdateEvent{
				ObjectOld: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									controller.RestartedAnnotation: "true",
								},
							},
						},
					},
				},
				ObjectNew: nil,
			},
			expUpdate: false,
		},
		{
			name: "both objects are nil",
			event: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: nil,
			},
			expUpdate: false,
		},
	}

	p := RestartDeploymentAnnotationPredicate{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			update := p.Update(test.event)
			g.Expect(update).To(Equal(test.expUpdate))
		})
	}
}
