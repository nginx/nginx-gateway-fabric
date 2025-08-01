package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestServiceChangedPredicate_Update(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		objectOld client.Object
		objectNew client.Object
		msg       string
		expUpdate bool
	}{
		{
			msg:       "nil objectOld",
			objectOld: nil,
			objectNew: &v1.Service{},
			expUpdate: false,
		},
		{
			msg:       "nil objectNew",
			objectOld: &v1.Service{},
			objectNew: nil,
			expUpdate: false,
		},
		{
			msg:       "non-Service objectOld",
			objectOld: &v1.Namespace{},
			objectNew: &v1.Service{},
			expUpdate: false,
		},
		{
			msg:       "non-Service objectNew",
			objectOld: &v1.Service{},
			objectNew: &v1.Namespace{},
			expUpdate: false,
		},
		{
			msg: "number of ports changed",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{},
				},
			},
			expUpdate: true,
		},
		{
			msg: "a target port changed",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       81,
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(82),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       81,
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(92), // this value changed
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			msg: "a service port changed",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       81,
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(82),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       91, // this value changed
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(82),
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			msg: "no ports changed",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       81,
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(82),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
						{
							Port:       81,
							TargetPort: intstr.FromInt(81),
						},
						{
							Port:       82,
							TargetPort: intstr.FromInt(82),
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			msg: "ports changed but service ports and target ports are the same",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
							Name:       "port",
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
							Name:       "not-port", // name is different
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			msg: "spec changed but ports are the same",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
				},
			},
			expUpdate: false,
		},
		{
			msg: "appProtocol changed",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							AppProtocol: helpers.GetPointer("oldAppProtocol"),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							AppProtocol: helpers.GetPointer("newAppProtocol"),
						},
					},
				},
			},
			expUpdate: true,
		},
		{
			msg: "appProtocol stayed the same",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							AppProtocol: helpers.GetPointer("sameAppProtocol"),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							AppProtocol: helpers.GetPointer("sameAppProtocol"),
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			msg: "multiple appProtocols stayed the same",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:        80,
							TargetPort:  intstr.FromInt(80),
							AppProtocol: helpers.GetPointer("sameAppProtocol80"),
						},
						{
							Port:        81,
							TargetPort:  intstr.FromInt(81),
							AppProtocol: helpers.GetPointer("sameAppProtocol81"),
						},
						{
							Port:        82,
							TargetPort:  intstr.FromInt(82),
							AppProtocol: helpers.GetPointer("sameAppProtocol82"),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:        80,
							TargetPort:  intstr.FromInt(80),
							AppProtocol: helpers.GetPointer("sameAppProtocol80"),
						},
						{
							Port:        81,
							TargetPort:  intstr.FromInt(81),
							AppProtocol: helpers.GetPointer("sameAppProtocol81"),
						},
						{
							Port:        82,
							TargetPort:  intstr.FromInt(82),
							AppProtocol: helpers.GetPointer("sameAppProtocol82"),
						},
					},
				},
			},
			expUpdate: false,
		},
		{
			msg: "multiple appProtocols with one changing",
			objectOld: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:        80,
							TargetPort:  intstr.FromInt(80),
							AppProtocol: helpers.GetPointer("sameAppProtocol80"),
						},
						{
							Port:        81,
							TargetPort:  intstr.FromInt(81),
							AppProtocol: helpers.GetPointer("sameAppProtocol81"),
						},
						{
							Port:        82,
							TargetPort:  intstr.FromInt(82),
							AppProtocol: helpers.GetPointer("sameAppProtocol82"),
						},
					},
				},
			},
			objectNew: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:        80,
							TargetPort:  intstr.FromInt(80),
							AppProtocol: helpers.GetPointer("sameAppProtocol80"),
						},
						{
							Port:        81,
							TargetPort:  intstr.FromInt(81),
							AppProtocol: helpers.GetPointer("sameAppProtocol81"),
						},
						{
							Port:        82,
							TargetPort:  intstr.FromInt(82),
							AppProtocol: helpers.GetPointer("differentAppProtocol"),
						},
					},
				},
			},
			expUpdate: true,
		},
	}

	p := ServiceChangedPredicate{}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			update := p.Update(event.UpdateEvent{
				ObjectOld: tc.objectOld,
				ObjectNew: tc.objectNew,
			})

			g.Expect(update).To(Equal(tc.expUpdate))
		})
	}
}

func TestServicePortsChangedPredicate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	p := ServiceChangedPredicate{}

	g.Expect(p.Delete(event.DeleteEvent{Object: &v1.Service{}})).To(BeTrue())
	g.Expect(p.Create(event.CreateEvent{Object: &v1.Service{}})).To(BeTrue())
	g.Expect(p.Generic(event.GenericEvent{Object: &v1.Service{}})).To(BeTrue())
}
