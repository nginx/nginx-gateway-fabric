# Securing Traffic using Let's Encrypt and Cert-Manager

Securing client server communication is a crucial part of modern application architectures. One of the most important
steps in this process is implementing HTTPS (HTTP over TLS/SSL) for all communications. This encrypts the data
transmitted between the client and server, preventing eavesdropping and tampering. To do this, you need an SSL/TLS
certificate from a trusted Certificate Authority (CA). However, issuing and managing certificates can be a complicated
manual process. Luckily, there are many services and tools available to simplify and automate certificate issuance and
management.

This guide will demonstrate how to:

- Configure HTTPS for your application using a [Gateway](https://gateway-api.sigs.k8s.io/api-types/gateway/).
- Use [Let’s Encrypt](https://letsencrypt.org) as the Certificate Authority (CA) issuing the TLS certificate.
- Use [cert-manager](https://cert-manager.io) to automate the provisioning and management of the certificate.

## Prerequisities

1. Administrator access to a Kubernetes cluster.
2. [Helm](https://helm.sh) and [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) must be installed locally.
3. Deploy NGINX Gateway Fabric (NGF) following the [deployment instructions](/docs/installation.md).
4. A DNS resolvable domain name is required. It must resolve to the public endpoint of the NGF deployment, and this
   public endpoint must be an external IP address or alias accessible over the internet. The process here will depend
   on your DNS provider. This DNS name will need to be resolvable from the Let’s Encrypt servers, which may require
   that you wait for the record to propagate before it will work.

## Overview

![cert-manager ACME Challenge and certificate management with Gateway API](/docs/images/cert-manager-gateway-workflow.png)

The diagram above shows a simplified representation of the cert-manager ACME Challenge and certificate issuance process
using Gateway API. Please note that not all of the Kubernetes objects created in this process are represented in
this diagram.

At a high level, the process looks like this:

1. We deploy cert-manager and create a ClusterIssuer which specifies Let’s Encrypt as our CA and Gateway as our ACME
   HTTP01 Challenge solver.
2. We create a Gateway resource for our domain (cafe.example.com) and configure cert-manager integration using an
   annotation.
3. This kicks off the certificate issuance process – cert-manager contacts Let’s Encrypt to obtain a certificate, and
   Let’s Encrypt starts the ACME challenge. As part of this challenge, a temporary HTTPRoute resource is created by
   cert-manager which directs the traffic through NGF to verify we control the domain name in the certificate request.
4. Once the domain has been verified, the certificate is issued. Cert-manager stores the keypair in a Kubernetes secret
   that is referenced by the Gateway resource. As a result, NGINX is configured to terminate HTTPS traffic from clients
   using this signed keypair.
5. We deploy our application and our HTTPRoute which defines our routing rules. The routing rules defined configure
   NGINX to direct requests to https://cafe.example.com/coffee to our coffee-app application, and to use the https
   Listener defined in our Gateway resource.
6. When the client connects to https://cafe.example.com/coffee, the request is routed to the coffee-app application
   and the communication is secured using the signed keypair contained in the cafe-secret Secret.
7. The certificate will be automatically renewed when it is close to expiry, the Secret will be updated using the new
   Certificate, and NGF will dynamically update the keypair on the filesystem used by NGINX for HTTPS termination once
   the Secret is updated.

## Details

### Step 1 – Deploy cert-manager

The first step is to deploy cert-manager onto the cluster.

- Add the Helm repository.

  ```shell
  helm repo add jetstack https://charts.jetstack.io
  helm repo update
  ```

- Install cert-manager, and enable the GatewayAPI feature gate:

  ```shell
  helm install \
    cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.12.0 \
    --set installCRDs=true \
    --set "extraArgs={--feature-gates=ExperimentalGatewayAPISupport=true}"
  ```

### Step 2 – Create a ClusterIssuer

Next we need to create a [ClusterIssuer](https://cert-manager.io/docs/concepts/issuer/), a Kubernetes resource that
represents the certificate authority (CA) that will generate the signed certificates by honouring certificate signing
requests.

We are using the ACME Issuer type, and Let's Encrypt as the CA server. In order for Let's Encypt to verify that we own
the domain a certificate is being requested for, we must complete "challenges". This is to ensure clients are
unable to request certificates for domains they do not own. We will configure the Issuer to use a HTTP01 challenge, and
our Gateway resource that we will create in the next step as the solver. To read more about HTTP01 challenges, see
[here](https://cert-manager.io/docs/configuration/acme/http01/). Use the following YAML definition to create the
resource, but please note the `email` field must be updated to your own email address.

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    # You must replace this email address with your own.
    # Let's Encrypt will use this to contact you about expiring
    # certificates, and issues related to your account.
    email: my-name@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      # Secret resource that will be used to store the account's private key.
      name: issuer-account-key
    # Add a single challenge solver, HTTP01 using NGF
    solvers:
    - http01:
        gatewayHTTPRoute:
          parentRefs: # This is the name of the Gateway that will be created in the next step
          - name: gateway
            namespace: default
            kind: Gateway
```

### Step 3 – Deploy our Gateway with the cert-manager annotation

Next we need to deploy our Gateway. Use can use the below YAML manifest, updating the `spec.listeners[1].hostname`
field to the required value for your environment.

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: gateway
  annotations: # This is the name of the ClusterIssuer created in the previous step
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    port: 80
    protocol: HTTP
  - name: https
    # Important: The hostname needs to be set to your domain
    hostname: "cafe.example.com"
    port: 443
    protocol: HTTPS
    tls:
      mode: Terminate
      certificateRefs:
      - kind: Secret
        name: cafe-secret
```

It's worth noting a couple of key details in this manifest:

- The cert-manager annotation is present in the metadata – this enables the cert-manager integration, and tells
  cert-manager which ClusterIssuer configuration it should use for the certificates.
- There are two Listeners configured, an HTTP Listener on port 80, and an HTTPS Listener on port 443.
  - The http Listener on port 80 is required for the HTTP01 ACME challenge to work. This is because as part of the
    HTTP01 Challenge, a temporary HTTPRoute will be created by cert-manager to solve the ACME challenge, and this
    HTTPRoute requires a Listener on port 80. See the [HTTP01 Gateway API solver documentation](https://cert-manager.io/docs/configuration/acme/http01/#configuring-the-http-01-gateway-api-solver)
    for more information.
  - The https Listener on port 443 is the Listener we will use in our HTTPRoute in the next step. Cert-manager will
    create a Certificate for this Listener block.
- The hostname needs to set to the required value. A new certificate will be issued from the `letsencrypt-prod`
  ClusterIssuer for the domain, e.g. "cafe.example.com", once the ACME challenge is successful.

Once the certificate has been issued, cert-manager will create a Certificate resource on the cluster and the
`cafe-secret` Secret containing the signed keypair in the same Namespace as the Gateway. We can verify the Secret has
been created successfully using `kubectl`. Note it will take a little bit of time for the Challenge to complete and the
Secret to be created:

```shell
kubectl get secret cafe-secret
```

```text
NAME          TYPE                DATA   AGE
cafe-secret   kubernetes.io/tls   2      20s
```

### Step 4 – Deploy our application and HTTPRoute
Now we can create our coffee Deployment and Service, and configure the routing rules. You can use the following manifest
to create the Deployment and Service:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coffee
spec:
  replicas: 1
  selector:
    matchLabels:
      app: coffee
  template:
    metadata:
      labels:
        app: coffee
    spec:
      containers:
      - name: coffee
        image: nginxdemos/nginx-hello:plain-text
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: coffee
spec:
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: coffee
```

Deploy our HTTPRoute to configure our routing rules for the coffee application. Note the `parentRefs` section in the
spec refers to the Listener configured in the previous step.

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: coffee
spec:
  parentRefs:
  - name: gateway
    sectionName: https
  hostnames: # Update the hostname to match what is configured in the Gateway resource
  - "cafe.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /coffee
    backendRefs:
    - name: coffee
      port: 80
```

## Testing

To test everything has worked correctly, we can use curl to the navigate to our endpoint, e.g.
https://cafe.example.com/coffee. To verify using curl, we can use the `-v` option to increase verbosity and inspect the
presented certificate. The output will look something like this:

```shell
curl https://cafe.example.com/coffee -v
```

```text
*   Trying 54.195.47.105:443...
* Connected to cafe.example.com (54.195.47.105) port 443 (#0)
* ALPN: offers h2,http/1.1
* (304) (OUT), TLS handshake, Client hello (1):
*  CAfile: /etc/ssl/cert.pem
*  CApath: none
* (304) (IN), TLS handshake, Server hello (2):
* (304) (IN), TLS handshake, Unknown (8):
* (304) (IN), TLS handshake, Certificate (11):
* (304) (IN), TLS handshake, CERT verify (15):
* (304) (IN), TLS handshake, Finished (20):
* (304) (OUT), TLS handshake, Finished (20):
* SSL connection using TLSv1.3 / AEAD-CHACHA20-POLY1305-SHA256
* ALPN: server accepted http/1.1
* Server certificate:
*  subject: CN=cafe.example.com
*  start date: Aug 11 08:22:11 2023 GMT
*  expire date: Nov  9 08:22:10 2023 GMT
*  subjectAltName: host "cafe.example.com" matched cert's "cafe.example.com"
*  issuer: C=US; O=Let's Encrypt; CN=R3
*  SSL certificate verify ok.
* using HTTP/1.1
> GET /coffee HTTP/1.1
> Host: cafe.example.com
> User-Agent: curl/7.88.1
> Accept: */*
>
< HTTP/1.1 200 OK
< Server: nginx/1.25.1
< Date: Fri, 11 Aug 2023 10:03:21 GMT
< Content-Type: text/plain
< Content-Length: 163
< Connection: keep-alive
< Expires: Fri, 11 Aug 2023 10:03:20 GMT
< Cache-Control: no-cache
<
Server address: 192.168.78.136:8080
Server name: coffee-9bf875848-xvkqv
Date: 11/Aug/2023:10:03:21 +0000
URI: /coffee
Request ID: e64c54a2ac253375ac085d48980f000a
* Connection #0 to host cafe.example.com left intact
```

## Troubleshooting

- For troubeshooting anything related to the cert-manager installation or Issuer setup, see
  [the cert-manager troubleshooting guide](https://cert-manager.io/docs/troubleshooting/).
- For troubleshooting the HTTP01 ACME Challenge, please see the cert-manager
  [ACME troubleshooting guide](https://cert-manager.io/docs/troubleshooting/acme/).
  - Note that for the HTTP01 Challenge to work using the Gateway resource, HTTPS redirect must not be configured.
  - The temporary HTTPRoute created by cert-manager routes the traffic between cert-manager and the Let's Encrypt server
    through NGF. If the Challenge is not successful, it may be useful to inspect the NGINX logs to see the ACME
    Challenge requests. You should see something like the following:

    ```shell
    kubectl logs <pod-name> -n nginx-gateway -c nginx
    <...>
    52.208.162.19 - - [15/Aug/2023:13:18:12 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "cert-manager-challenges/v1.12.0 (linux/amd64) cert-manager/bd192c4f76dd883f9ee908035b894ffb49002384"
    52.208.162.19 - - [15/Aug/2023:13:18:14 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "cert-manager-challenges/v1.12.0 (linux/amd64) cert-manager/bd192c4f76dd883f9ee908035b894ffb49002384"
    52.208.162.19 - - [15/Aug/2023:13:18:16 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "cert-manager-challenges/v1.12.0 (linux/amd64) cert-manager/bd192c4f76dd883f9ee908035b894ffb49002384"
    52.208.162.19 - - [15/Aug/2023:13:18:18 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "cert-manager-challenges/v1.12.0 (linux/amd64) cert-manager/bd192c4f76dd883f9ee908035b894ffb49002384"
    52.208.162.19 - - [15/Aug/2023:13:18:20 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "cert-manager-challenges/v1.12.0 (linux/amd64) cert-manager/bd192c4f76dd883f9ee908035b894ffb49002384"
    3.128.204.81 - - [15/Aug/2023:13:18:22 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "Mozilla/5.0 (compatible; Let's Encrypt validation server; +https://www.letsencrypt.org)"
    23.178.112.204 - - [15/Aug/2023:13:18:22 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "Mozilla/5.0 (compatible; Let's Encrypt validation server; +https://www.letsencrypt.org)"
    35.166.192.222 - - [15/Aug/2023:13:18:22 +0000] "GET /.well-known/acme-challenge/bXQn27Lenax2AJKmOOS523T-MWOKeFhL0bvrouNkUc4 HTTP/1.1" 200 87 "-" "Mozilla/5.0 (compatible; Let's Encrypt validation server; +https://www.letsencrypt.org)"
    <...>
    ```

## Links

- Gateway docs: https://gateway-api.sigs.k8s.io
- Cert-manager Gateway usage: https://cert-manager.io/docs/usage/gateway/
- Cert-manager ACME: https://cert-manager.io/docs/configuration/acme/
- Let’s Encrypt: https://letsencrypt.org
- NGINX HTTPS docs: https://docs.nginx.com/nginx/admin-guide/security-controls/terminating-ssl-http/
