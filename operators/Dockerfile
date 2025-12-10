FROM quay.io/operator-framework/helm-operator:v1.42.0

# Update system packages to fix vulnerabilities
USER root
RUN microdnf update -y && microdnf clean all
USER 1001

COPY LICENSE /licenses/LICENSE

LABEL name="nginx-gateway-fabric-operator" \
      maintainer="kubernetes@nginx.com" \
      vendor="F5 NGINX" \
      version="1.0.2" \
      release="1" \
      summary="NGINX Gateway Fabric Operator" \
      description="Helm-based operator for NGINX Gateway Fabric"

ENV HOME=/opt/helm
COPY operators/watches.yaml ${HOME}/watches.yaml
COPY charts ${HOME}/helm-charts
COPY config ${HOME}/config
WORKDIR ${HOME}
