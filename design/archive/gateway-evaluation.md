# Gateway Evaluation

STATUS: Archived

This document captures the design of the initial experimental work to implement
the [Gateway API](https://gateway-api.sigs.k8s.io/) using the NGINX data plane. Throughout this document we will refer
to this work as "Gateway evaluation".

## Motivation

The SIG-NETWORK group has begun a mission to improve traffic control to and from Kubernetes clusters. The Gateway API
aims to improve service networking by formalizing user personas and developing a decoupled and composable collection of
resources for provisioning and configuring a data plane (load balancer). The intent is to create an extensible and
expressive API that lends itself naturally to role-based access control methods.

## Requirements

### Personas

Gateway API focuses on three main personas: Infrastructure Provider (Infra), Cluster Operator (Admin), and Application
Developer (AppDev); for more details
see [Roles and personas](https://gateway-api.sigs.k8s.io/concepts/security-model/#roles-and-personas).

The Infra role focuses on provider tier assets: compute (e.g. virtual machines, base-metal server/node hardware),
storage (e.g. external arrays, SANs), and networking (e.g. VPNs, subnet isolation, routing).

NGINX does not provide Infra services or a platform product as it pertains to this role. For the purposes of this
evaluation we intend to *focus on the **Admin** and **AppDev** personas*. The Gateway evaluation will require (and in
some cases assume) a correct infrastructure and network topology is in place; cloud network routing, SDN control, DNS,
and L4 load balancing are out of scope.

### Resources

The Gateway evaluation intends to solve for a subset of L7 traffic management use-cases.

Three main API kinds will be supported for chosen Core features with Optional and Extended features to be considered
later:

- GatewayClass: Defines a set of gateways with a common configuration and behavior.
- Gateway: Requests a point where traffic can be translated to Services within the cluster.
- Routes: Describe how traffic coming via the Gateway maps to the Services.

The Gateway evaluation will employ each in the following manner:

- GatewayClass: The Admin controls the resource lifecycle and is responsible for creating, updating, and deleting a
  GatewayClass for NGINX. The GatewayClass is used by the Admin to provide global configuration; by way of
  ParameterRefs, to the subordinate Gateway resources.
- Gateway: The Admin controls the resource lifecycle and is responsible for creating, updating, and deleting a Gateway
  for NGINX. The Gateway resource is a representation and description of an NGINX data plane ready to be configured with
  Routes.
- Routes: The Gateway API describes multiple typed Routes: HTTP, TLS, TCP, and UDP. The AppDev controls the resource
  lifecycle and is responsible for creating, updating, and deleting a Route. The Gateway evaluation will only support
  HTTPRoute resources, and only an initial subset of features and not the entire Core set.

Gateway evaluation requires supplemental resources; other Core API resources are needed to complete traffic routing
between an ingress point and the final backend Pod endpoints. Required supplemental resources are:

- Services: The AppDev controls the resource lifecycle and is responsible for creating, updating, and deleting Service
  resources. Kubernetes Service resources describe a logical collection of like processes and act as a load balancing
  primitive. AppDevs create Service resources to describe their product subsystems, Services act as an abstraction for a
  set of real backend processes that can serve requests.
- Endpoints: Kubernetes controllers manage Endpoint resources, Endpoints provide the association between Service
  abstractions and real servers. Gateway evaluation will watch Endpoints to discover upstream addresses. When AppDevs
  create Service and HTTPRoute objects, Gateway evaluation uses references in the HTTPRoute (via a HTTPRouteRules and
  HTTPBackendRefs (see [HTTP Routing](https://gateway-api.sigs.k8s.io/guides/http-routing/) for more detail)) to
  discover the Pod IP endpoints, i.e. Gateway evaluation uses Endpoints resources, referred to by HTTPRoutes, to link
  routing rules to upstream addresses.

### Goals

- Vet a minimal feature set of Core resources for their applicability and conformance to NGINX configuration models.
- Support L7 HTTPRoute resources; host routing and path routing only.
- Support GatewayClass, Gateway, and HTTPRoute Status subobject requirements.
- Develop expertise in Gateway API and refine a compatible, extensible, and flexible architecture.

### Non-Goals

- Replace or supplant current [NGINX Ingress Controller](https://github.com/nginx/kubernetes-ingress) (NIC) use-cases
  and will not work to achieve feature parity.
- The Gateway evaluation will not support Ingress v1 APIs or the set of NIC CRDs (e.g. VirtualServer and
  VirtualServerRoute).
- The Gateway will only support Service resources as a proper HTTPBackendRef.

## Design

Gateway evaluation is a decoupled (two containers) control plane and data plane.

The data plane is NGINX OSS run as a container within a Kubernetes Pod with `.spec.shareProcessNamespace` set to true.
The control plane will interact with the data plane to provide configuration and limited process control.

The control plane manages Kubernetes controllers; each controller follows
the [Controller pattern](https://kubernetes.io/docs/concepts/architecture/controller/#controller-pattern).

General dataflow:

```text
-----------    ---------------    ---------------    ---------------------------    -----------------------
| K8S API | -> | Controllers | -> | Reconcilers | -> | Configuration Subsystem | -> | NGINX Data Plane    |
-----------    ---------------    ---------------    | Conf Graph / AST / IR   |    | Configuration Write |
                                                     ---------------------------    -----------------------
                                                        |                              ^
                                                        |                              |
                                                        |------- Process Signal -------|
```

The above shows a general data flow for the Gateway evaluation. Admins and AppDevs interact directly with the Kubernetes
API server and the Gateway evaluation provides no additional network APIs or out-of-band control mechanisms for user
interactions (excluding environment and CLI startup arguments). Using the controller pattern, Gateway evaluation reacts
to and reconciles configuration updates. The Gateway API primitives are translated to compatible internal data model
representations of NGINX data plane configurations. The resultant configurations are sent to data plane components and
actuated via data plane natively supported means (writing to disk, API calls, process signaling, etc.).

Gateway evaluation will be deployed inside the Kubernetes cluster using Deployments, DaemonSets, or unmanaged Pods and
must be a pair of containers - the data plane container and the control plane container. (TBD: leader election,
controller-runtime provides facilities but requirements need defining.)

Expected deployment:

```text
External        |  Cluster
Network         |  Network
                |                        --------------
                |                        | API Server |
                |                        --------------
                |                          |
                |                          V
                |                        ---------------------
                |                        | ----------------  |
                |                        | | control plane | |         ------------
                |                        | ----------------  |       - | Upstream |
                |                        |   |               |      /  ------------
                |                        |   V               |     /
------------    |    ----------------    | -------------     |    /    ------------
| Ext.Load | -> | -> | Svc Endpoint | -> | | data plane | -> | ------- | Upstream |
| Balancer |    |    ----------------    | -------------     |    \    ------------
------------    |                        ---------------------     \
                |                                                   \  ------------
                |                                                    - | Upstream |
                |                                                      ------------
```

### Prerequisites

- Infra persona has enabled external access to the cluster. For example, configured an external network device which has
  connectivity to the cluster Node underlay network.
- Admin has knowledge of the cluster external DNS Hostname and listen Port.

### Deployment Requirements

- Admin provisions GatewayClass referencing Gateway object name.
- Admin provisions Gateway using name referenced from GatewayClass.
- Admin provisions Gateway using hostname and port: `.spec.listeners[].{ hostname, port, protocol }` where protocol MUST
  be "HTTP".
- Admin provisions cluster access to NGINX data planes via Service abstractions: Service `.spec.type=LoadBalancer`
  or `.spec.type=NodePort`.
- Admin provisions Gateway evaluation deployment via workload abstractions, Deployment or DaemonSet. Admin MAY provision
  single Pod via an unmanaged Pod configuration.

### Operation

> ***NOTE***
>
> *Where applicable Gateway evaluation will write status updates to Gateway API resources according to specification
requirements. Each update is not recorded in these flows.*

#### Startup

At process startup, the Gateway evaluation must discover the "state of the world". Until startup constraints have been
satisified Gateway evaluation data plane instances will be unable to proxy data. Data plane instances MUST be
inaccessible until GatewayClass, Gateway resources are properly resolved, i.e., GatewayClass and Gateway resources gate
data flow. Gateway evaluation MAY use a combination of Kubernetes readinessProbes and NGINX configuration to prevent
traffic.

Gateway evaluation will,

- initialize with a minimal configuration; reading an environment variable or command-line argument referencing the
  Gateway's controller name (the controller name will match the Gateway.Name and the GatewayClass.Controller fields).
- find its GatewayClass.
  - list GatewayClass resources.
  - match GatewayClass.Controller to Gateway evaluation's controller name (conflict resolution: oldest wins).
  - watch and wait for GatewayClass creation and updates.
  - log its state and subsequent state change (GatewayClass found or not found, GatewayClass discovered and process
      proceeding).
- watch for ParametersRef: `.spec.parametersRef`. If not found, continue processing with default runtime configuration.
- find its Gateway.
  - list Gateway resources.
  - match Gateway.Name to Gateway evaluation's controller name (conflict resolution: oldest wins).
  - watch and wait for Gateway creation and updates.
  - log its state and subsequent state change (Gateway found or not found, Gateway discovered and process proceeding).
- bootstrap internal data models with `.spec.listeners[].hostname` for route matching.
- instantiate controller and control loop for HTTPRoute resources.
- instantiate controller and control loop for Services/Endpoints resources.
- rebuild current declarative state using stored HTTPRoute, Services/Endpoints resources.
- enter runtime loop.

#### Runtime

The Gateway evaluation is primarily a set of control loops watching for changes in the declared configuration of the
Gateway API.

Process runtime configuration updates may be delivered by updates to GatewayClass, Gateway, and ParametersRef. Gateway
evaluation will watch these resources for updates, reconciling their changes, and resolving or rejecting based on
calculated conflicts.

Data plane traffic decisions and configuration are primarily conveyed via HTTPRoute updates. Gateway evaluation will
establish an HTTPRoute controller reconciling configuration updates, and resolving or rejecting based on calculated
conflicts or validation errors. HTTPRoute resources describe the server side proxy configuration using a set of rules
applied to backend references. For the Gateway evaluation, only Service references will be supported. Gateway evaluation
will establish additional controllers for Service and Endpoints updates. Updates (HTTPRoute and Endpoints) are
translated into an intermediate representation describing the NGINX server, location, and upstreams; data structure
specifics are yet to determined. Properties desired: sparse, directed, acyclic, relational.

Once validated for conflicts and errors, the intermediate representation is exported to the data plane as NGINX
configuration directives.

Loop cycle:

- Event notification: Upsert, Delete
- Reconcilation:
  - Service/Endpoints, HTTPRoute:
    - Delete:
      - Remove data structure nodes.
      - Validate, commit or rollback.
      - Write config.
    - Upsert:
      - Add or Update data structure nodes.
      - Validate, commit or rollback.
      - Write config.
  - Gateway:
    - Delete:
      - Gateway is disabled.
    - Upsert:
      - Add or Update data structure nodes.
      - Validate, commit or rollback.
      - Write config.
  - GatewayClass:
    - Delete:
      - Gateway is disabled.
    - Upsert:
      - Reconfigure control plane.
  - ParametersRef:
    - Delete:
      - Gateway reverts to default runtime.
    - Upsert:
      - Reconfigure control plane and data plane elements.

### Security Considerations

Gateway evaluation will not support ReferencePolicy resources and will not support cross namespace references. All
references are to be validated against the parent resource's namespace and only allowed when the resources reside in the
same isolation boundary.
See [Object references](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#object-references).

## Appendix A

> ***NOTE***
>
> *This is not a design requirement and may not be met as a delivery constraint.*

Controller design:

```text
---------------    ------------------    -----------------------    ----------------------
| Controllers | -> | SDK Reconciler | -> | SDK Impl. Reconcile | -> | Graph / AST / Conf |
---------------    ------------------    -----------------------    ----------------------
                     ^                     ^
                     |                     |
                   -------               -------------
                   | SDK | <- Register - | SDK Impl. |
                   -------               -------------
```

Controllers can be defined using Go interfaces and a modified Pimpl idiom (which Go naturally supports via structural
typing and the interface paradigm).

With this approach controllers are built with a public set of interfaces and are responsible for all event watch
operations. Implementations are separated into individual packages which satisfy the public interfaces. In the above
diagram, SDK represents a package which provides access to versioned implementations, public interfaces, and the core
network machinery of a controller reconciliation loop.

E.g.)

```go
package sdk

type V1Alpha2Impl interface{
  Upsert(*runtime.Object)
}

type sdk struct {
  impl V1Alpha2Impl
}

func (s sdk) Register(i V1Alpha1Impl) {
  ...
}

func (s sdk) V1Alpha2() V1Alpha2Impl {
  ...
}
```

SDK controllers reconcile and call through to backend implementations:

```go
func (c controller) Reconcile(...) {
  sdk.V1Alpha2().Upsert(...)
}
```

where an implementer will satisfy the interface and register their implementation during startup:

```go
type myImpl {
}

func (m *myImpl) Upsert(o *runtime.Object) {
  ...
}

func main() {
  m := myImpl{}

  sdk.Register(m)
}
```
