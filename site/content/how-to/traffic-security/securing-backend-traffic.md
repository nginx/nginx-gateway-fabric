---
title: "Securing backend traffic"
weight: 200
toc: true
type: how-to
product: NGF
docs: "DOCS-1423"
---

Learn how to encrypt HTTP traffic between NGINX Gateway Fabric and your backend pods.

---

## Overview

In this guide, we will show how to specify the TLS configuration of the connection from the Gateway to a backend pod/s via the Service API object using a [BackendTLSPolicy](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/). This covers the use-case where the service or backend owner is doing their own TLS and NGINX Gateway Fabric needs to know how to connect to this backend pod that has its own certificate over HTTPS.

---

## Note on Gateway API Experimental Features

{{< important >}} BackendTLSPolicy is a Gateway API resource from the experimental release channel. {{< /important >}}

{{<include "installation/install-gateway-api-experimental-features.md" >}}

---

## Before you begin

- [Install]({{< relref "installation/" >}}) NGINX Gateway Fabric with experimental features enabled.
- Save the public IP address and port of NGINX Gateway Fabric into shell variables:

   ```text
   GW_IP=XXX.YYY.ZZZ.III
   GW_PORT=<port number>
   ```

{{< note >}}In a production environment, you should have a DNS record for the external IP address that is exposed, and it should refer to the hostname that the gateway will forward for.{{< /note >}}

---

## Set up

Create the **secure-app** application in Kubernetes by copying and pasting the following block into your terminal:

```yaml
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secure-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: secure-app
  template:
    metadata:
      labels:
        app: secure-app
    spec:
      containers:
        - name: secure-app
          image: nginxinc/nginx-unprivileged:latest
          ports:
            - containerPort: 8443
          volumeMounts:
            - name: secret
              mountPath: /etc/nginx/ssl
              readOnly: true
            - name: config-volume
              mountPath: /etc/nginx/conf.d
      volumes:
        - name: secret
          secret:
            secretName: app-tls-secret
        - name: config-volume
          configMap:
            name: secure-config
---
apiVersion: v1
kind: Service
metadata:
  name: secure-app
spec:
  ports:
    - port: 8443
      targetPort: 8443
      protocol: TCP
      name: https
  selector:
    app: secure-app
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: secure-config
data:
  app.conf: |-
    server {
      listen 8443 ssl;
      listen [::]:8443 ssl;

      server_name secure-app.example.com;

      ssl_certificate /etc/nginx/ssl/tls.crt;
      ssl_certificate_key /etc/nginx/ssl/tls.key;

      default_type text/plain;

      location / {
        return 200 "hello from pod secure-app\n";
      }
    }
---
apiVersion: v1
kind: Secret
metadata:
  name: app-tls-secret
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUQwRENDQXJpZ0F3SUJBZ0lVVDQwYTFYd3doUHVBdDJNMkdZZUovYXluZlFBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1JqRWZNQjBHQTFVRUF3d1djMlZqZFhKbExXRndjQzVsZUdGdGNHeGxMbU52YlRFTE1Ba0dBMVVFQmhNQwpWVk14RmpBVUJnTlZCQWNNRFZOaGJpQkdjbUZ1YzJselkyOHdIaGNOTWpRd01URTRNVGd3TVRBeFdoY05NalV3Ck1URTNNVGd3TVRBeFdqQi9NUXN3Q1FZRFZRUUdFd0pWVXpFVE1CRUdBMVVFQ0F3S1EyRnNhV1p2Y201cFlURVcKTUJRR0ExVUVCd3dOVTJGdUlFWnlZVzV6YVhOamJ6RU9NQXdHQTFVRUNnd0ZUa2RKVGxneEVqQVFCZ05WQkFzTQpDVTVIU1U1WUlFUmxkakVmTUIwR0ExVUVBd3dXYzJWamRYSmxMV0Z3Y0M1bGVHRnRjR3hsTG1OdmJUQ0NBU0l3CkRRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQkFMeUx0eURNbTZ4M0ZEUFJsOGZ0azNweCtrRWQKYTVpTGZOQ3lDbUVjYktBQVBDNEhZckl5b1B5QXpSTlJCMWErekE0UTlrbzJZRG5vR0dkeFJaMEdydldKZUV2Mgo3MWlHNGxhbHRVTS9WOWNvSktQY0UyTEI0R3R6cFA3ckdIWXNvRDlOUXFpV3YwZ0lOdE42MjdrWGg4UW41V1hYCk92Y2FkS2h0bjJER3RvU0VzT3dpNzR5NEt3SmFkWnlwLzJaM0hPakRTNjVIVmxydmUxUXpBMVRzTEp6S3cva3gKbHBSR0lWK0lhUjZXbXZsaVFVdDJxWFg0L3hGeVVEM2Vic05TeXpHUk5mQ0NOTWxlWlV3MTR3ZUdhOEVnc2tDcQprOGdYSmpFZXQxMlR4OGxkY3BpVWlxYVpkOStYZjJmUS8yL2Y5c1IzM3Q4K0VVUWpoZ2ZIbHlsLzV1RUNBd0VBCkFhTjlNSHN3SHdZRFZSMGpCQmd3Rm9BVTRUT096c1d0Q3ZWdGJlWXFSU0FqN2tXajFkb3dDUVlEVlIwVEJBSXcKQURBTEJnTlZIUThFQkFNQ0JQQXdJUVlEVlIwUkJCb3dHSUlXYzJWamRYSmxMV0Z3Y0M1bGVHRnRjR3hsTG1OdgpiVEFkQmdOVkhRNEVGZ1FVZmtWREFFWmIwcjRTZ2swck10a0FvQ2c2RjRnd0RRWUpLb1pJaHZjTkFRRUxCUUFECmdnRUJBQWFiQit6RzVSODl6WitBT2RsRy9wWE9nYjF6VkJsQ0dMSkhyYTl1cTMvcXRPR1VacDlnd2dZSWJ4VnkKUkVLbWVRa05pV0haSDNCSlNTZ3czbE9abGNxcW5xbUJ2OFAxTUxDZ3JqbDJSN1d2NVhkb2RlQkJxc0lvZkNxVgp3ZG51THJUU3RTbmd2MGhDcldBNlBmTnlQeXMzSGJva1k3RExNREhuNmhBQWcwMUNDT0pWWGpNZjFqLzNIMFNCClBQSWxtek5aRUpEd0JMR2hyb1V3aUY3NkNUV1Fudi8yc1pvWHMwUlFiRTY3TmNraXc2Z0svaWRwVTVzMmlkOEQKVExjVjNxenVFaE1ZeUlua0ZWNEJLZlFkTWxDQnE1QWdyU1Jqb2FoaCszbFRwYVpUalJGUGFVd3VZYXVsQXRzNgpra1ROaGltWWQ3Ym1aVk5MK2I0MzhmN1RMaGc9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
  tls.key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2Z0lCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktnd2dnU2tBZ0VBQW9JQkFRQzhpN2Nnekp1c2R4UXoKMFpmSDdaTjZjZnBCSFd1WWkzelFzZ3BoSEd5Z0FEd3VCMkt5TXFEOGdNMFRVUWRXdnN3T0VQWktObUE1NkJobgpjVVdkQnE3MWlYaEw5dTlZaHVKV3BiVkRQMWZYS0NTajNCTml3ZUJyYzZUKzZ4aDJMS0EvVFVLb2xyOUlDRGJUCmV0dTVGNGZFSitWbDF6cjNHblNvYlo5Z3hyYUVoTERzSXUrTXVDc0NXbldjcWY5bWR4em93MHV1UjFaYTczdFUKTXdOVTdDeWN5c1A1TVphVVJpRmZpR2tlbHByNVlrRkxkcWwxK1A4UmNsQTkzbTdEVXNzeGtUWHdnalRKWG1WTQpOZU1IaG12QklMSkFxcFBJRnlZeEhyZGRrOGZKWFhLWWxJcW1tWGZmbDM5bjBQOXYzL2JFZDk3ZlBoRkVJNFlICng1Y3BmK2JoQWdNQkFBRUNnZ0VBUXJucGJleXJmVTVKTW91Ty9UenBrQkJ0UWdVZzhvUVBBS2E1d0tONEYrbnQKWWxiUHlZUGNjSEErNDRLdUo3ZHZiTno0NU11NG8xV3Q2Vkh2a29KdWdjd01iRW53YTdLVXdKaDFmVjZaL2pXaApQZkpoVS9hTUwwcm1qaWJ5YWNRaVZEVEtEZk1Ici96a05sVEpGUWlzVGpIV1lBUGJSTjh5Z1BjR3pBK1hRVzg5CmxsOFdoeWwrZndjRVAzNTM4TUdKYUpHVmV4Q2d5cVRyKzZwQ29yRUpFL1pSNytiMTNyejRsbmxpZXVYa2pCVkcKSnYvUVI0RVhTSDhuRit3K2FvWTFQaGd2QnFJTnFQZjJMT1V0MzNiN3FDTkFmSVBRdFZFejJsN3NqbmJlcElTTwpvTkhNUFY0N21XTzY2dzhFMzJVMjNjVEtUbytJcWovM0d1eGhweXlYaHdLQmdRRDVNRDhmY3ZyM0xVZHkvZ3I0Ci82MVBqUXNSaWRYdjN0Mk1MVEk4UkduNzJWcGxvVVExNDZHV2xGTGVVVDY1L1ZVMjNsZFUrUWt2eFNMK3U1bW4KRUJIdXUyVmtBUWVYcUJXWkpPTmFSZG9Ia1YzK2Fyc0U4Qld0SWVtZHJ0MWN6bHFjc1VzMWdGdG1COGI3RHB3UwpHKzRoZDlzZG0weDBNS2hoOFVGOUQrZytUd0tCZ1FEQnN4dUpoZ1hnck14S0dVMHJ5MW9WWTJtMGpDUEpJcTkzCmNZRUZGY3lYZ0U4OWlDVmlob3dVVE8yMXpTZ3o0SVVKNUxoc2M5N3VIVER1VXdwcVI5NFBsdjlyaXJvakowM1UKT3FyWHgwbWdNN2xibVM1L1RwS0czZG1QblZ0WEZMektISFgyWDVnUW56emYvdXVKK3NtbDVLQW5WN0VZc1oxcgpkVXJvRm8zcnp3S0JnUUNZdjM5aUdzeEdLaVpMRWZqTjY0UmthRFBwdTFFOTZhSnE0K1dRVmV1VnF3V2ptTGhFClJGWHdCTm5MVjRnWTRIYVUzTFF4N1RvNVl5RnhmclBRV2FSMGI4RFdEVitIRWt5ekJJNnM3bmFZL3YzY0Q3YTIKYnlrS2FPaFlkVEZTUzFmMkJ5UHdGczl2K3NKNWNOb3dxNWhNUWJrNks5RXd4QWJqaXN5M0NjSTJOd0tCZ1FDbwo2c2pZNVVlNjV2WkFxRS9rSVRJdDlNUDU3enhGNnptWnNDSVRqUzhkNzRjcTRjKzRYQjFNbHNtMkFYTk55ajQ2Cm9uc3lHTm9RVE9TZThVdmo0MGlEeitwdW5rdzAyOUhEZ21YNlJwQ3VaRzBBdEZVWU1DMFg3K0FLbmU5SndZdmgKdFhBcHFyT3h5eXdMS3dPOUVEZEp0RmIxK0VNNGhhd0NTZ2RJM21KbGdRS0JnRzIxeEJNRXRzMFBVN3lDYTZ0YwpadDc1NUV4aEdkR3F5MmtHYmtmdzBEaHBQQVVUZmdncVF3NVBYdGVIS1ZBSDlKaG5kVnBBZFFxNmZ1MER1MDNKCkl0cGpxNWluZXVoR0x0alpMR1Nhd0dwY0FUU3h4Z3dCM0l0Z29LKzBCRFhteWxId0lEcUc5Z2crRU5KK0VhL0MKeTFOMmV0ZG1sQ01hNjM4cVJlNFlTWk55Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0=

EOF
```

This will create the **secure-app** service and a deployment, as well as a Secret containing the certificate and key that will be used by the backend application to decrypt the HTTPS traffic. Note that the application is configured to accept HTTPS traffic only. Run the following command to verify the resources were created:

```shell
kubectl get pods,svc
```

Your output should include the **secure-app** pod and the **secure-app** service:

```text
NAME                          READY   STATUS      RESTARTS   AGE
pod/secure-app-868cfd5b5-v7gwk   1/1     Running   0          9s

NAME                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
service/secure-app   ClusterIP   10.96.213.57   <none>        8443/TCP  9s
```

---

## Configure routing rules

First, we will create the Gateway resource with an HTTP listener:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    port: 80
    protocol: HTTP
EOF
```

Next, we will create our HTTPRoute to route traffic to our secure-app backend:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: secure-app
spec:
  parentRefs:
  - name: gateway
    sectionName: http
  hostnames:
  - "secure-app.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: secure-app
      port: 8443
EOF
```

---

## Send Traffic without backend TLS configuration

Using the external IP address and port for NGINX Gateway Fabric, we can send traffic to our secure-app application. To show what happens if we send plain HTTP traffic from NGF to our `secure-app`, let's try sending a request before we create the backend TLS configuration.

{{< note >}}If you have a DNS record allocated for `secure-app.example.com`, you can send the request directly to that hostname, without needing to resolve.{{< /note >}}

```shell
curl --resolve secure-app.example.com:$GW_PORT:$GW_IP http://secure-app.example.com:$GW_PORT/
```

```text
<html>
<head><title>400 The plain HTTP request was sent to HTTPS port</title></head>
<body>
<center><h1>400 Bad Request</h1></center>
<center>The plain HTTP request was sent to HTTPS port</center>
<hr><center>nginx/1.25.3</center>
</body>
</html>
```

We can see we a status 400 Bad Request message from NGINX.

---

## Create the backend TLS configuration

To configure the backend TLS terminationm, first we will create the ConfigMap that holds the `ca.crt` entry for verifying our self-signed certificates:

```yaml
kubectl apply -f - <<EOF
kind: ConfigMap
apiVersion: v1
metadata:
  name: backend-cert
data:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDbTCCAlWgAwIBAgIUPA3fFnkLl63GZ7noUjb5NoLhSYkwDQYJKoZIhvcNAQEL
    BQAwRjEfMB0GA1UEAwwWc2VjdXJlLWFwcC5leGFtcGxlLmNvbTELMAkGA1UEBhMC
    VVMxFjAUBgNVBAcMDVNhbiBGcmFuc2lzY28wHhcNMjQwMTE4MTgwMTAxWhcNMjUw
    MTA4MTgwMTAxWjBGMR8wHQYDVQQDDBZzZWN1cmUtYXBwLmV4YW1wbGUuY29tMQsw
    CQYDVQQGEwJVUzEWMBQGA1UEBwwNU2FuIEZyYW5zaXNjbzCCASIwDQYJKoZIhvcN
    AQEBBQADggEPADCCAQoCggEBAJGgn81BrqzmI4aQmGrg7RgkO5oYwlThQ9X/xVHB
    YVFptjRPAZz9g92g5birI/NZ43C6nEbZrJrSCqN3wgvV84jJmBAgpAvW+LhF4caa
    nhAnecJCcTbwrd542vCDoDRsNV5ffbpESgC4FxPGkRVbSa0KHQz8qCLqS2+uaB7X
    t76iw6y4pQ3klobVp1XtUpzZMGMBqZFnsAdl+PWMmSTvqjixkSlfcUY6Crnk9W6d
    Sns5cpzKdUs+2ZkBe6VkBgSs8xbaz8Y2YC1GhRqGlxYLT3WBaIlSCKPuRrGjwE3r
    AsW6gSL919H1O1a+MjQuLuQ4lnCbCpNzM9OV1JISMWfwifMCAwEAAaNTMFEwHQYD
    VR0OBBYEFOEzjs7FrQr1bW3mKkUgI+5Fo9XaMB8GA1UdIwQYMBaAFOEzjs7FrQr1
    bW3mKkUgI+5Fo9XaMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEB
    AG/eX4pctINIrHvRyHOusdac5iXSJbQRZgWvP1F2p95qoIDESciAU1Sh1oJv+As5
    IlJOZPJNuZFpDLjc8kzSoEbc1Q5+QyTBlyNNsagWYYwK0CEJ6KJt80vytffmdOIg
    z8/a+2Ax829vcn1w1SUi5V6ea/l8K74f2SL/zSSHgtEiz8V0TlvT7J6wurgmnk4t
    yQRmsXlDGefuijMNCVf7jWwLx2BODfKoEA1pJkthnNvdizlikmz+9elxhV9bRf3Y
    NnubytWPfO1oeHjVGvxVjCouIYine+VlskvwHmMi/dYod6yd7aFYu4CU3g/hjwKo
    LY2WNv5j3JhDnEYK9Zj3z7A=
    -----END CERTIFICATE-----
EOF
```

Next, we create the Backend TLS Policy which targets our `secure-app` Service and refers to the ConfigMap created in the previous step:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1alpha3
kind: BackendTLSPolicy
metadata:
  name: backend-tls
spec:
  targetRefs:
  - group: ''
    kind: Service
    name: secure-app
  validation:
    caCertificateRefs:
    - name: backend-cert
      group: ''
      kind: ConfigMap
    hostname: secure-app.example.com
EOF
```

To confirm the Policy was created and attached successfully, we can run a describe on the BackendTLSPolicy object:

```shell
kubectl describe backendtlspolicies.gateway.networking.k8s.io
```

```text
Name:         backend-tls
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.networking.k8s.io/v1alpha3
Kind:         BackendTLSPolicy
Metadata:
  Creation Timestamp:  2024-05-15T12:02:38Z
  Generation:          1
  Resource Version:    19380
  UID:                 b3983a6e-92f1-4a98-b2af-64b317d74528
Spec:
  Target Refs:
    Group:
    Kind:       Service
    Name:       secure-app
  Validation:
    Ca Certificate Refs:
      Group:
      Kind:    ConfigMap
      Name:    backend-cert
    Hostname:  secure-app.example.com
Status:
  Ancestors:
    Ancestor Ref:
      Group:      gateway.networking.k8s.io
      Kind:       Gateway
      Name:       gateway
      Namespace:  default
    Conditions:
      Last Transition Time:  2024-05-15T12:02:38Z
      Message:               BackendTLSPolicy is accepted by the Gateway
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

---

## Send traffic with backend TLS configuration

Now let's try sending traffic again:

```shell
curl --resolve secure-app.example.com:$GW_PORT:$GW_IP http://secure-app.example.com:$GW_PORT/
```

```text
hello from pod secure-app
```

---

## See also

To learn more about configuring backend TLS termination using the Gateway API, see the following resources:

- [Backend TLS Policy](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/)
- [Backend TLS Policy GEP](https://gateway-api.sigs.k8s.io/geps/gep-1897/)
