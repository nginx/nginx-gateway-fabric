### Local

1. Generate a local public and private key using OpenSSL

```
openssl genpkey -algorithm RSA -out dev/private_key.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -in dev/pivate_key.pem -pubout -out public_key.pem
```

2. Run `make jwks` 

	This will:
	- Create a secret called `jwt-keys-secure`
	- If that secret already exists, it will delete it and re-create it with a new JWKS

3. Run `make jwt`

	This will:
	- Create a JSON Web Token and save it to `$TOKEN`

4. Deploy NGF with NGINX Plus

5. Deploy JWT AuthenticationFilter: `kubectl apply -f example/jwt-file-auth/`

6. `curl --resolve cafe.example.com:8080:127.0.0.1 http://cafe.example.com:8080/tea -H "Authorization: Bearer $TOKEN"`

### Remote

Log in to the NGINX private registry, pull the image and load it into KIND

```
docker login private-registry.nginx.com/
docker pull private-registry.nginx.com/nginx-plus/base:alpine --platform linux/x86_64
kind load docker-image private-registry.nginx.com/nginx-plus/base:alpine
```

Deploy NGINX Plus in Kubernetes:

1. `cd dev/nginx`
2. `export LICENSE_FILE=</path/to/license.jwt>`
3. `make deploy-plus`

If you have any issues, run `make cleanup` to cleanup the environment

Port forward NGINX

```
make port-forward &
```


#### Keycloak configuration

1. Deploy Keycloak

```
kubectl apply -f dev/keycloak.yaml
```

Note: This pod may take some time to be ready.

2. Port-forward the keycloak service:

```
kubectl -n default port-forward svc/keycloak 18080:8080
```

3. Go to http://localhost:18080/  - Login with username `admin` and password `admin`

Create a new client called `nginx-client`

- On the 2nd page, ensure
	- `Direct access grants` are enabled. This will allow us to get the access token from this client later.
	- `Client authentication` and `Authorization` are enabled. This is required to allow us to access the client secret. This will be needed later.

Click Next, and, on the 3rd page, click Save

4. Navigate to the Credentials tab of the `nginx-client` and copy the `Client Secret` to your clipboard

Save this to the `CLIENT_SECRET` env variable

```
CLIENT_SECRET=<client's secret>
```

5. Create a new user called `nginx-user`.  Add an email and first & last name as well. This is to ensure the user's profile is fully set up. If not, this will block us from obtaining the access token. These can be set to any value.

5. After creating the user, go to the Credentials tab and select `Set Password`
This can be whatever you want. I used `nginx`

Save this password to the `$PASSWORD` env variable

6. Run this CURL command to get the Client's token, and save it to `CLIENT_TOKEN`

```
export CLIENT_TOKEN=$(curl -k -L -X POST 'http://localhost:18080/realms/master/protocol/openid-connect/token' \
-H 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode grant_type=password \
--data-urlencode scope=openid \
--data-urlencode client_id=nginx-client \
--data-urlencode client_secret=$CLIENT_SECRET \
--data-urlencode username=nginx-user \
--data-urlencode password=$PASSWORD \
| jq -r .access_token)
```

7. Test CURL

```
curl -v localhost:8080/auth -H "Authorization: Bearer ${CLIENT_TOKEN}"
```