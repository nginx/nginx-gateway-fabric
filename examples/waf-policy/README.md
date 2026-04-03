# F5 WAF for NGINX example

This directory contains the YAML files used in the [F5 WAF Integration](https://docs.nginx.com/nginx-gateway-fabric/traffic-security/f5waf-integration/) guide.

## Local testing steps (temp)

Test environment: GKE

### Install NGF

- Create the required secrets and install the gateway crds. Ensure you have a `dockerconfig.jwt` in the repo root with the JWT token for pulling the private registry images (required for the NAP images), and a `license.jwt` for the nginx plus license requirements.

```bash
# Install the gateway CRDS and create the required secrets
make install-gateway-crds
kubectl create ns nginx-gateway || true
# Need license.jwt in the root of the NGF repo dir
kubectl -n nginx-gateway create secret generic nplus-license --from-file license.jwt || true
make create-image-pull-secret
```

- Build the images and push them to an accessible registry:

```bash
NGINX_IMAGE_REPO=<repo>
NGF_IMAGE_REPO=<repo>
NGF_IMAGE_TAG=nap-waf
make build-images-with-nap-waf GOARCH=amd64 PREFIX=$NGF_IMAGE_REPO NGINX_PLUS_PREFIX=$NGINX_IMAGE_REPO TAG=$NGF_IMAGE_TAG
docker push $NGINX_IMAGE_REPO:$NGF_IMAGE_TAG
docker push $NGF_IMAGE_REPO:$NGF_IMAGE_TAG
```

- Install using helm:

```bash
NGF_CHART_PATH=<path-to-chart>
helm install nginx-gateway $NGF_CHART_PATH \
  --wait \
  --namespace nginx-gateway --create-namespace \
  --set nginx.image.pullPolicy=Always \
  --set nginx.imagePullSecret=nginx-plus-registry-secret \
  --set nginx.usage.endpoint=product.connect.nginxlab.net \
  --set nginx.plus=true \
  --set nginx.image.repository=$NGINX_IMAGE_REPO \
  --set nginx.image.tag=$NGF_IMAGE_TAG \
  --set nginxGateway.image.repository=$NGF_IMAGE_REPO \
  --set nginxGateway.image.tag=$NGF_IMAGE_TAG \
  --set nginxGateway.image.pullPolicy=Always \
  --set nginxGateway.productTelemetry.enable=false
```

### NIM

#### Install NIM in the cluster

- Create a values.yaml file

```yaml
nmsClickhouse:
  mode: disabled

imagePullSecrets:
  - name: regcred

apigw:
  image:
    repository: private-registry.nginx.com/nms/apigw
  service:
    type: LoadBalancer
core:
  image:
    repository: private-registry.nginx.com/nms/core
dpm:
  image:
    repository: private-registry.nginx.com/nms/dpm
ingestion:
  image:
    repository: private-registry.nginx.com/nms/ingestion
integrations:
  image:
    repository: private-registry.nginx.com/nms/integrations
secmon:
  image:
    repository: private-registry.nginx.com/nms/secmon
utility:
  image:
    repository: private-registry.nginx.com/nms/utility
```

- Create an image pull secret for NIM

As previously, ensure you have a `dockerconfig.jwt` in the repo root with the JWT token for pulling the private registry images. The secret should exist in the nim namespace and be called `regcred`

```bash
kubectl create namespace nim
kubectl create secret docker-registry regcred \
  --docker-server=private-registry.nginx.com \
  --docker-username=$(cat dockerconfig.jwt) \
  --docker-password=none \
  -n nim
```

- Add the repo

```bash
helm repo add nginx-stable https://helm.nginx.com/stable
helm repo update
```

and install, setting the admin password to a value of your choosing

```bash
helm install nim nginx-stable/nim \
  -n nim \
  --create-namespace \
  --set adminPasswordHash=$(openssl passwd -6 'admin') \
  -f values.yaml \
  --version 2.1.1 \
  --wait
```

#### Log into the UI and register the license

Retrieve the external IP for the NIM apigw service

```bash
kubectl get svc -n nim apigw
```

You should see something like the following:

```text
NAME    TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)         AGE
apigw   LoadBalancer   7.6.5.4         1.2.3.4        443:31986/TCP   110m
```

Use this external IP, e.g. `https://1.2.3.4/login`, and the password created in the install step to log into the UI

Once logged in, add your `license.jwt` file (available from myf5) to register the instance - click the settings cog in the top right hand corner, go to licenses, and drag and drop the license file.

#### Configure the policies

- Navigate back to the main dashboard by clicking the F5 NGINX logo in the top left corner
- Go to WAF -> Policies
- Click the elipsis for each of the pre defined policies and click `compile`
- Optionally, add an additional policy and compile that too

### Apply the examples

- Apply `cafe.yaml`, `cafe-routes.yaml`, `nim-creds.yaml`, `nginx-proxy.yaml`, `syslog.yaml`, and `gateway.yaml`. Check the pods are running and that the Gateway pod and has 3 containers:

```bash
kubectl get pods
```

```text
NAME                             READY   STATUS    RESTARTS   AGE
coffee-569775f75-5npzs           1/1     Running   0          38h
gateway-nginx-5bbc87d6c5-ksk6k   3/3     Running   0          118m
syslog-b9db868b7-hg9pt           1/1     Running   0          38h
tea-75bc9f4b6d-xlldd             1/1     Running   0          37h
```

- Save the GW_IP and GW_PORT into variables - get the address from the `gateway` resource:

```text
k get gateway
NAME      CLASS   ADDRESS   PROGRAMMED   AGE
gateway   nginx   1.2.3.4   True         44h
```

- Send a malicious appearing URL to `tea` - notice we get a normal response

```text
curl --resolve cafe.example.com:$GW_PORT:$GW_IP "http://cafe.example.com:$GW_PORT/tea"
Server address: 10.0.2.4:8080
Server name: tea-75bc9f4b6d-xlldd
Date: 01/Apr/2026:10:29:43 +0000
URI: /tea
Request ID: 354d7c697d97377dcc66ad73a0e28eff
```

- Apply the `wafpolicy.yaml` resource and check the conditions. Once ready, you should see something like:

```text
Conditions:
      Last Transition Time:  2026-04-01T08:44:10Z
      Message:               The Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2026-04-01T08:44:10Z
      Message:               All references are resolved
      Observed Generation:   1
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
      Last Transition Time:  2026-04-01T08:44:10Z
      Message:               Policy is programmed in the data plane
      Observed Generation:   1
      Reason:                Programmed
      Status:                True
      Type:                  Programmed
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
```

- Run the same `tea` curl again and you should see something like

```text
curl --resolve cafe.example.com:$GW_PORT:$GW_IP "http://cafe.example.com:$GW_PORT/tea/<script>"
<html><head><title>Request Rejected</title></head><body>The requested URL was rejected. Please consult with your administrator.<br><br>Your support ID is: 15122839540880170981<br><br><a href='javascript:history.back();'>[Go Back]</a></body></html>%
```

- Check the syslog logs (change the pod name to match your env)

```bash
kubectl exec -it syslog-867f7777d-cwcss -- cat /var/log/messages
```

we should see something like (output truncated here)

```text
VIOL_BOT_CLIENT</viol_name></violation><violation><viol_index>93</viol_index><viol_name>VIOL_RATING_THREAT</viol_name></violation></request-violations></BAD_MSG>",bot_signature_name="curl",bot_category="HTTP Library",bot_anomalies="N/A",enforced_bot_anomalies="N/A",client_class="Untrusted Bot",client_application="N/A",client_application_version="N/A",request="GET /tea/<script> HTTP/1.1\r\nHost: cafe.example.com\r\nUser-Agent: curl/8.7.1\r\nAccept: */*\r\n\r\n",transport_protocol="HTTP/1.1"
```

### In cluster file server

pending instructions
