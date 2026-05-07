package cache

import (
	"testing"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/configmaps"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

func TestTransformGatewayClass(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	controllerName := "example.com/gateway-controller"

	gc := &gatewayv1.GatewayClass{
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: gatewayv1.GatewayController(controllerName),
		},
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "foo"}},
		},
	}

	tr := TransformGatewayClass(controllerName)
	res, err := tr(gc)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resGC, ok := res.(*gatewayv1.GatewayClass)
	g.Expect(ok).To(BeTrue())
	g.Expect(resGC.Spec.ControllerName).To(Equal(gatewayv1.GatewayController(controllerName)))
	g.Expect(resGC.ManagedFields).To(BeNil())

	// Non-matching controller name returns stripped GatewayClass (preserves cache key integrity)
	gc2 := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "other-gc",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "bar"}},
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: "other-controller",
		},
	}
	res, err = tr(gc2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resGC2, ok := res.(*gatewayv1.GatewayClass)
	g.Expect(ok).To(BeTrue())
	g.Expect(resGC2.Name).To(Equal("other-gc"))
	g.Expect(resGC2.Spec).To(Equal(gatewayv1.GatewayClassSpec{}))
	g.Expect(resGC2.ManagedFields).To(BeNil())

	// Not a GatewayClass returns the object as-is
	secretObj := &corev1.Secret{}
	res, err = tr(secretObj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(Equal(secretObj))
}

func TestTransformSecret(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tr := TransformSecret()

	// Secret with relevant keys
	secret := &corev1.Secret{
		Data: map[string][]byte{
			secrets.LicenseJWTKey:      []byte("jwt"),
			secrets.CAKey:              []byte("ca"),
			secrets.TLSCertKey:         []byte("cert"),
			secrets.TLSKeyKey:          []byte("key"),
			secrets.N1CDataplaneKey:    []byte("dataplane.key"),
			corev1.DockerConfigJsonKey: []byte("docker"),
			corev1.DockerConfigKey:     []byte("docker2"),
			secrets.ClientSecretKey:    []byte("client-secret"),
			secrets.CRLKey:             []byte("crl"),
			"irrelevant":               []byte("nope"),
		},
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "foo"}},
		},
	}
	res, err := tr(secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resSecret, ok := res.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())

	// Only relevant keys remain
	g.Expect(resSecret.Data).To(HaveKey(secrets.LicenseJWTKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.CAKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.TLSCertKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.TLSKeyKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.N1CDataplaneKey))
	g.Expect(resSecret.Data).To(HaveKey(corev1.DockerConfigJsonKey))
	g.Expect(resSecret.Data).To(HaveKey(corev1.DockerConfigKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.ClientSecretKey))
	g.Expect(resSecret.Data).To(HaveKey(secrets.CRLKey))
	g.Expect(resSecret.Data).ToNot(HaveKey("irrelevant"))
	g.Expect(resSecret.ManagedFields).To(BeNil())

	// Secret with no relevant keys returns stripped Secret (preserves cache key integrity)
	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "irrelevant-secret",
			Namespace:     "default",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "bar"}},
		},
		Data: map[string][]byte{"foo": []byte("bar")},
	}
	res, err = tr(secret2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resSecret2, ok := res.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(resSecret2.Name).To(Equal("irrelevant-secret"))
	g.Expect(resSecret2.Namespace).To(Equal("default"))
	g.Expect(resSecret2.Data).To(BeNil())
	g.Expect(resSecret2.ManagedFields).To(BeNil())

	// Not a Secret returns the object as-is
	cmObj := &corev1.ConfigMap{}
	res, err = tr(cmObj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(Equal(cmObj))
}

func TestTransformConfigMap(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tr := TransformConfigMap()

	// All relevant keys
	keys := []string{
		secrets.CAKey,
		configmaps.AgentConfKey,
		configmaps.MainConfKey,
		configmaps.EventsConfKey,
	}

	// ConfigMap with all relevant keys in Data and BinaryData, plus irrelevant keys
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			secrets.CAKey:            "ca-data",
			configmaps.AgentConfKey:  "agent-data",
			configmaps.MainConfKey:   "main-data",
			configmaps.EventsConfKey: "events-data",
			"irrelevant":             "nope",
		},
		BinaryData: map[string][]byte{
			secrets.CAKey:            []byte("ca-bin"),
			configmaps.AgentConfKey:  []byte("agent-bin"),
			configmaps.MainConfKey:   []byte("main-bin"),
			configmaps.EventsConfKey: []byte("events-bin"),
			"irrelevant":             []byte("nope"),
		},
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "foo"}},
		},
	}
	res, err := tr(cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM, ok := res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())

	// Only relevant keys remain
	for _, k := range keys {
		g.Expect(resCM.Data).To(HaveKey(k))
		g.Expect(resCM.BinaryData).To(HaveKey(k))
	}
	g.Expect(resCM.Data).ToNot(HaveKey("irrelevant"))
	g.Expect(resCM.BinaryData).ToNot(HaveKey("irrelevant"))
	g.Expect(resCM.ManagedFields).To(BeNil())

	// ConfigMap with only one relevant key in Data
	cm2 := &corev1.ConfigMap{
		Data: map[string]string{configmaps.MainConfKey: "main-data"},
	}
	res, err = tr(cm2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM, ok = res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(resCM.Data).To(Equal(map[string]string{"main.conf": "main-data"}))
	g.Expect(resCM.BinaryData).To(BeNil())

	// ConfigMap with only one relevant key in BinaryData
	cm3 := &corev1.ConfigMap{
		BinaryData: map[string][]byte{configmaps.EventsConfKey: []byte("ev-bin")},
	}
	res, err = tr(cm3)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM, ok = res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(resCM.Data).To(BeNil())
	g.Expect(resCM.BinaryData).To(Equal(map[string][]byte{"events.conf": []byte("ev-bin")}))

	// ConfigMap with no relevant keys returns stripped ConfigMap (preserves cache key integrity)
	cm4 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "irrelevant-cm",
			Namespace:     "default",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "bar"}},
		},
		Data:       map[string]string{"foo": "bar"},
		BinaryData: map[string][]byte{"baz": []byte("qux")},
	}
	res, err = tr(cm4)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM4, ok := res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(resCM4.Name).To(Equal("irrelevant-cm"))
	g.Expect(resCM4.Namespace).To(Equal("default"))
	g.Expect(resCM4.Data).To(BeNil())
	g.Expect(resCM4.BinaryData).To(BeNil())
	g.Expect(resCM4.ManagedFields).To(BeNil())

	// Not a ConfigMap returns the object as-is
	secretObj := &corev1.Secret{}
	res, err = tr(secretObj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(Equal(secretObj))
}
