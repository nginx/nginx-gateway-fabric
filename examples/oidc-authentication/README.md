## Testing Setup

This section describes how to set up a local environment to test OIDC authentication with Keycloak and NGINX Gateway Fabric.

---

## Setup Keycloak

Start the Keycloak instance using Docker Compose:

```bash
docker-compose up -d
```

To allow the host to resolve the Keycloak container hostname, add the following entry to your host machine:

```bash
echo "127.0.0.1 host.docker.internal" | sudo tee -a /etc/hosts
```

Access the Keycloak admin console at:

```text
http://host.docker.internal:8180/admin/master/console
```

Follow the next steps:

Create a new **Realm** named:

```text
nginx-gateway
```

Create a new **Client**:

  - Record the **Client ID** and **Client Secret**.
  - Enable the following authentication flows: Standard flow and Direct Access Grants

Configure the client settings:

**Redirect URLs**

Ensure the redirect URL uses the port where the Gateway will be exposed externally. Avoid using port `8443` because it conflicts with Keycloak.

Example:

```text
https://cafe.example.com:8442/*
```

**Web Origins**

```text
https://cafe.example.com
```

Create a test user in the `nginx-gateway` realm:

- Enable **email verification**
- Under **Credentials**, set a password
- Disable **Temporary** so the password can be used directly for testing

---

## Scenario 1: Testing with Self-Signed Certificates

The following steps generate a local Certificate Authority (CA) and sign certificates for both Keycloak and NGINX.

Generate a Certificate Authority

```bash
openssl genrsa -out ca.key 4096

openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 \
  -out ca.crt \
  -subj "/CN=LocalCA"
```

Generate Keycloak Certificate (Signed by CA)

```bash
openssl genrsa -out keycloak.key 4096

openssl req -new -key keycloak.key -out keycloak.csr \
  -subj "/CN=host.docker.internal"

openssl x509 -req -in keycloak.csr \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out keycloak.crt -days 365 \
  -extfile <(printf "subjectAltName=DNS:host.docker.internal,DNS:localhost,IP:127.0.0.1")
```

Generate NGINX Certificate for `cafe.example.com`

```bash
openssl genrsa -out nginx.key 4096

openssl req -new -key nginx.key -out nginx.csr \
  -subj "/CN=cafe.example.com"

openssl x509 -req -in nginx.csr \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out nginx.crt -days 365 \
  -extfile <(printf "subjectAltName=DNS:cafe.example.com")
```

Create a Kubernetes secret containing the CA certificate so NGF/NGINX can verify the Keycloak TLS certificate.

```bash
kubectl apply -f client-secret.yaml
```

Create the following resources:

1. Gateway

   - Configure an HTTPS listener in Terminate mode.
   - Reference the TLS certificate and key used by NGINX.

2. Coffee Application

3. Keycloak Client Secret

   - Create a Kubernetes secret using the **Client Secret** from the Keycloak client configuration

4. OIDC AuthenticationFilter

5. HTTPRoute

   - Reference the OIDC filter


## Expose the Gateway

Expose the Gateway locally using port forwarding. Example:

```bash
kubectl port-forward svc/gateway 8442:443
```

Update the `NginxProxy` to resolve the KubeDNS address:

```bash
  dnsResolver:
    addresses:
    - type: IPAddress
      value: 10.96.0.10
```

---

## Test the Authentication Flow

Open the following URL in your browser:

```text
https://cafe.example.com:8442/coffee
```

You should be redirected to Keycloak for authentication. After successfully logging in, the request will be redirected back to the application through the configured OIDC flow.

**Note** : All required files should be in this folder. This will need cleanup when we finally merge OIDC into main.
