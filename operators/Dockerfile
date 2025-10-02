FROM quay.io/operator-framework/helm-operator:v1.41.1

COPY LICENSE /licenses/LICENSE

LABEL name="nginx-gateway-fabric-operator" \
      vendor="F5 NGINX" \
      version="1.0.0" \
      release="1" \
      summary="NGINX Gateway Fabric Operator" \
      description="Helm-based operator for NGINX Gateway Fabric"

ENV HOME=/opt/helm
COPY operators/watches.yaml ${HOME}/watches.yaml
COPY charts ${HOME}/helm-charts
COPY config ${HOME}/config
WORKDIR ${HOME}
