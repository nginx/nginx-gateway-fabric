---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Deploy x to '...' using some.yaml
2. View logs on '....'
3. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Your environment**
* Version of the NGINX Gateway Fabric - release version or a specific commit. The first line of the nginx-gateway container logs includes the commit info.
* Version of Kubernetes
* Kubernetes platform (e.g. Mini-kube or GCP)
* Details on how you expose the NGINX Gateway Fabric Pod (e.g. Service of type LoadBalancer or port-forward)
* Logs of NGINX container: `kubectl -n <nginx-deployment-namespace> logs deployments/<nginx-deployment>`
* NGINX Configuration: `kubectl -n <nginx-deployment-namespace> exec -it deployments/<nginx-deployment> -- nginx -T`

**Additional context**
Add any other context about the problem here. Any log files you want to share.
