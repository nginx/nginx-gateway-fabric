package cache

import (
	"testing"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

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

	// Non-matching controller name
	gc2 := &gatewayv1.GatewayClass{
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: "other-controller",
		},
	}
	res, err = tr(gc2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())

	// Not a GatewayClass
	res, err = tr(&corev1.Secret{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())
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
			corev1.DockerConfigJsonKey: []byte("docker"),
			corev1.DockerConfigKey:     []byte("docker2"),
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
	g.Expect(resSecret.Data).To(HaveKey(corev1.DockerConfigJsonKey))
	g.Expect(resSecret.Data).To(HaveKey(corev1.DockerConfigKey))
	g.Expect(resSecret.Data).ToNot(HaveKey("irrelevant"))
	g.Expect(resSecret.ManagedFields).To(BeNil())

	// Secret with no relevant keys
	secret2 := &corev1.Secret{
		Data: map[string][]byte{"foo": []byte("bar")},
	}
	res, err = tr(secret2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())

	// Not a Secret
	res, err = tr(&corev1.ConfigMap{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())
}

func TestTransformConfigMap(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tr := TransformConfigMap()

	// ConfigMap with CAKey in Data
	cm := &corev1.ConfigMap{
		Data:       map[string]string{secrets.CAKey: "ca-data", "other": "x"},
		BinaryData: map[string][]byte{"foo": []byte("bar")},
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "foo"}},
		},
	}
	res, err := tr(cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM, ok := res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(resCM.Data).To(Equal(map[string]string{secrets.CAKey: "ca-data"}))
	g.Expect(resCM.BinaryData).To(BeNil())
	g.Expect(resCM.ManagedFields).To(BeNil())

	// ConfigMap with CAKey in BinaryData
	cm2 := &corev1.ConfigMap{
		BinaryData: map[string][]byte{secrets.CAKey: []byte("bin-ca")},
	}
	res, err = tr(cm2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).ToNot(BeNil())
	resCM, ok = res.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(resCM.Data).To(BeNil())
	g.Expect(resCM.BinaryData).To(Equal(map[string][]byte{secrets.CAKey: []byte("bin-ca")}))

	// ConfigMap with no CAKey
	cm3 := &corev1.ConfigMap{
		Data: map[string]string{"foo": "bar"},
	}
	res, err = tr(cm3)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())

	// Not a ConfigMap
	res, err = tr(&corev1.Secret{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeNil())
}
