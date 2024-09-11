---
title: "API reference"
weight: 100
toc: false
---
## Overview
NGINX Gateway API Reference
<p>Packages:</p>
<ul>
<li>
<a href="#gateway.nginx.org%2fv1alpha1">gateway.nginx.org/v1alpha1</a>
</li>
</ul>
<h2 id="gateway.nginx.org/v1alpha1">gateway.nginx.org/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains API Schema definitions for the
gateway.nginx.org API group.</p>
</p>
Resource Types:
<ul><li>
<a href="#gateway.nginx.org/v1alpha1.ClientSettingsPolicy">ClientSettingsPolicy</a>
</li><li>
<a href="#gateway.nginx.org/v1alpha1.NginxGateway">NginxGateway</a>
</li><li>
<a href="#gateway.nginx.org/v1alpha1.NginxProxy">NginxProxy</a>
</li><li>
<a href="#gateway.nginx.org/v1alpha1.ObservabilityPolicy">ObservabilityPolicy</a>
</li></ul>
<h3 id="gateway.nginx.org/v1alpha1.ClientSettingsPolicy">ClientSettingsPolicy
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ClientSettingsPolicy" title="Permanent link">¶</a>
</h3>
<p>
<p>ClientSettingsPolicy is an Inherited Attached Policy. It provides a way to configure the behavior of the connection
between the client and NGINX Gateway Fabric.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
gateway.nginx.org/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClientSettingsPolicy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientSettingsPolicySpec">
ClientSettingsPolicySpec
</a>
</em>
</td>
<td>
<p>Spec defines the desired state of the ClientSettingsPolicy.</p>
<br/>
<br/>
<table class="table table-bordered table-striped">
<tr>
<td>
<code>body</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientBody">
ClientBody
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Body defines the client request body settings.</p>
</td>
</tr>
<tr>
<td>
<code>keepAlive</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAlive">
ClientKeepAlive
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeepAlive defines the keep-alive settings.</p>
</td>
</tr>
<tr>
<td>
<code>targetRef</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#LocalPolicyTargetReference">
sigs.k8s.io/gateway-api/apis/v1alpha2.LocalPolicyTargetReference
</a>
</em>
</td>
<td>
<p>TargetRef identifies an API object to apply the policy to.
Object must be in the same namespace as the policy.
Support: Gateway, HTTPRoute, GRPCRoute.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#PolicyStatus">
sigs.k8s.io/gateway-api/apis/v1alpha2.PolicyStatus
</a>
</em>
</td>
<td>
<p>Status defines the state of the ClientSettingsPolicy.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxGateway">NginxGateway
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxGateway" title="Permanent link">¶</a>
</h3>
<p>
<p>NginxGateway represents the dynamic configuration for an NGINX Gateway Fabric control plane.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
gateway.nginx.org/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>NginxGateway</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.NginxGatewaySpec">
NginxGatewaySpec
</a>
</em>
</td>
<td>
<p>NginxGatewaySpec defines the desired state of the NginxGateway.</p>
<br/>
<br/>
<table class="table table-bordered table-striped">
<tr>
<td>
<code>logging</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Logging">
Logging
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Logging defines logging related settings for the control plane.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.NginxGatewayStatus">
NginxGatewayStatus
</a>
</em>
</td>
<td>
<p>NginxGatewayStatus defines the state of the NginxGateway.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxProxy">NginxProxy
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxProxy" title="Permanent link">¶</a>
</h3>
<p>
<p>NginxProxy is a configuration object that is attached to a GatewayClass parametersRef. It provides a way
to configure global settings for all Gateways defined from the GatewayClass.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
gateway.nginx.org/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>NginxProxy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.NginxProxySpec">
NginxProxySpec
</a>
</em>
</td>
<td>
<p>Spec defines the desired state of the NginxProxy.</p>
<br/>
<br/>
<table class="table table-bordered table-striped">
<tr>
<td>
<code>ipFamily</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.IPFamilyType">
IPFamilyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamily specifies the IP family to be used by the NGINX.
Default is &ldquo;dual&rdquo;, meaning the server will use both IPv4 and IPv6.</p>
</td>
</tr>
<tr>
<td>
<code>telemetry</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Telemetry">
Telemetry
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Telemetry specifies the OpenTelemetry configuration.</p>
</td>
</tr>
<tr>
<td>
<code>rewriteClientIP</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.RewriteClientIP">
RewriteClientIP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RewriteClientIP defines configuration for rewriting the client IP to the original client&rsquo;s IP.</p>
</td>
</tr>
<tr>
<td>
<code>disableHTTP2</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableHTTP2 defines if http2 should be disabled for all servers.
Default is false, meaning http2 will be enabled for all servers.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ObservabilityPolicy">ObservabilityPolicy
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ObservabilityPolicy" title="Permanent link">¶</a>
</h3>
<p>
<p>ObservabilityPolicy is a Direct Attached Policy. It provides a way to configure observability settings for
the NGINX Gateway Fabric data plane. Used in conjunction with the NginxProxy CRD that is attached to the
GatewayClass parametersRef.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>
gateway.nginx.org/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ObservabilityPolicy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ObservabilityPolicySpec">
ObservabilityPolicySpec
</a>
</em>
</td>
<td>
<p>Spec defines the desired state of the ObservabilityPolicy.</p>
<br/>
<br/>
<table class="table table-bordered table-striped">
<tr>
<td>
<code>tracing</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Tracing">
Tracing
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tracing allows for enabling and configuring tracing.</p>
</td>
</tr>
<tr>
<td>
<code>targetRefs</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#LocalPolicyTargetReference">
[]sigs.k8s.io/gateway-api/apis/v1alpha2.LocalPolicyTargetReference
</a>
</em>
</td>
<td>
<p>TargetRefs identifies the API object(s) to apply the policy to.
Objects must be in the same namespace as the policy.
Support: HTTPRoute, GRPCRoute.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#PolicyStatus">
sigs.k8s.io/gateway-api/apis/v1alpha2.PolicyStatus
</a>
</em>
</td>
<td>
<p>Status defines the state of the ObservabilityPolicy.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Address">Address
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Address" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.RewriteClientIP">RewriteClientIP</a>)
</p>
<p>
<p>Address is a struct that specifies address type and value.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.AddressType">
AddressType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type specifies the type of address.
Default is &ldquo;cidr&rdquo; which specifies that the address is a CIDR block.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Value specifies the address value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.AddressType">AddressType
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.AddressType" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Address">Address</a>)
</p>
<p>
<p>AddressType specifies the type of address.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;cidr&#34;</p></td>
<td><p>AddressTypeCIDR specifies that the address is a CIDR block.
kubebuilder:validation:Pattern=<code>^[\.a-zA-Z0-9:]*(\/([0-9]?[0-9]?[0-9]))$</code></p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ClientBody">ClientBody
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ClientBody" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientSettingsPolicySpec">ClientSettingsPolicySpec</a>)
</p>
<p>
<p>ClientBody contains the settings for the client request body.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>maxSize</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Size">
Size
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxSize sets the maximum allowed size of the client request body.
If the size in a request exceeds the configured value,
the 413 (Request Entity Too Large) error is returned to the client.
Setting size to 0 disables checking of client request body size.
Default: <a href="https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size">https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size</a>.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Duration">
Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout defines a timeout for reading client request body. The timeout is set only for a period between
two successive read operations, not for the transmission of the whole request body.
If a client does not transmit anything within this time, the request is terminated with the
408 (Request Time-out) error.
Default: <a href="https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_timeout">https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_timeout</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ClientKeepAlive">ClientKeepAlive
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ClientKeepAlive" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientSettingsPolicySpec">ClientSettingsPolicySpec</a>)
</p>
<p>
<p>ClientKeepAlive defines the keep-alive settings for clients.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>requests</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requests sets the maximum number of requests that can be served through one keep-alive connection.
After the maximum number of requests are made, the connection is closed. Closing connections periodically
is necessary to free per-connection memory allocations. Therefore, using too high maximum number of requests
is not recommended as it can lead to excessive memory usage.
Default: <a href="https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_requests">https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_requests</a>.</p>
</td>
</tr>
<tr>
<td>
<code>time</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Duration">
Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Time defines the maximum time during which requests can be processed through one keep-alive connection.
After this time is reached, the connection is closed following the subsequent request processing.
Default: <a href="https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_time">https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_time</a>.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAliveTimeout">
ClientKeepAliveTimeout
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout defines the keep-alive timeouts for clients.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ClientKeepAliveTimeout">ClientKeepAliveTimeout
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ClientKeepAliveTimeout" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAlive">ClientKeepAlive</a>)
</p>
<p>
<p>ClientKeepAliveTimeout defines the timeouts related to keep-alive client connections.
Default: <a href="https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_timeout">https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_timeout</a>.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>server</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Duration">
Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Server sets the timeout during which a keep-alive client connection will stay open on the server side.
Setting this value to 0 disables keep-alive client connections.</p>
</td>
</tr>
<tr>
<td>
<code>header</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Duration">
Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Header sets the timeout in the &ldquo;Keep-Alive: timeout=time&rdquo; response header field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ClientSettingsPolicySpec">ClientSettingsPolicySpec
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ClientSettingsPolicySpec" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientSettingsPolicy">ClientSettingsPolicy</a>)
</p>
<p>
<p>ClientSettingsPolicySpec defines the desired state of ClientSettingsPolicy.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>body</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientBody">
ClientBody
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Body defines the client request body settings.</p>
</td>
</tr>
<tr>
<td>
<code>keepAlive</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAlive">
ClientKeepAlive
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeepAlive defines the keep-alive settings.</p>
</td>
</tr>
<tr>
<td>
<code>targetRef</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#LocalPolicyTargetReference">
sigs.k8s.io/gateway-api/apis/v1alpha2.LocalPolicyTargetReference
</a>
</em>
</td>
<td>
<p>TargetRef identifies an API object to apply the policy to.
Object must be in the same namespace as the policy.
Support: Gateway, HTTPRoute, GRPCRoute.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ControllerLogLevel">ControllerLogLevel
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ControllerLogLevel" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Logging">Logging</a>)
</p>
<p>
<p>ControllerLogLevel type defines the logging level for the control plane.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;debug&#34;</p></td>
<td><p>ControllerLogLevelDebug is the debug level for control plane logging.</p>
</td>
</tr><tr><td><p>&#34;error&#34;</p></td>
<td><p>ControllerLogLevelError is the error level for control plane logging.</p>
</td>
</tr><tr><td><p>&#34;info&#34;</p></td>
<td><p>ControllerLogLevelInfo is the info level for control plane logging.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Duration">Duration
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Duration" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientBody">ClientBody</a>,
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAlive">ClientKeepAlive</a>,
<a href="#gateway.nginx.org/v1alpha1.ClientKeepAliveTimeout">ClientKeepAliveTimeout</a>,
<a href="#gateway.nginx.org/v1alpha1.TelemetryExporter">TelemetryExporter</a>)
</p>
<p>
<p>Duration is a string value representing a duration in time.
Duration can be specified in milliseconds (ms), seconds (s), minutes (m), hours (h).
A value without a suffix is seconds.
Examples: 120s, 50ms, 5m, 1h.</p>
</p>
<h3 id="gateway.nginx.org/v1alpha1.IPFamilyType">IPFamilyType
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.IPFamilyType" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxProxySpec">NginxProxySpec</a>)
</p>
<p>
<p>IPFamilyType specifies the IP family to be used by NGINX.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;dual&#34;</p></td>
<td><p>Dual specifies that NGINX will use both IPv4 and IPv6.</p>
</td>
</tr><tr><td><p>&#34;ipv4&#34;</p></td>
<td><p>IPv4 specifies that NGINX will use only IPv4.</p>
</td>
</tr><tr><td><p>&#34;ipv6&#34;</p></td>
<td><p>IPv6 specifies that NGINX will use only IPv6.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Logging">Logging
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Logging" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxGatewaySpec">NginxGatewaySpec</a>)
</p>
<p>
<p>Logging defines logging related settings for the control plane.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>level</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.ControllerLogLevel">
ControllerLogLevel
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Level defines the logging level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxGatewayConditionReason">NginxGatewayConditionReason
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxGatewayConditionReason" title="Permanent link">¶</a>
</h3>
<p>
<p>NginxGatewayConditionReason defines the set of reasons that explain why a
particular NginxGateway condition type has been raised.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Invalid&#34;</p></td>
<td><p>NginxGatewayReasonInvalid is a reason that is used with the &ldquo;Valid&rdquo; condition when the condition is False.</p>
</td>
</tr><tr><td><p>&#34;Valid&#34;</p></td>
<td><p>NginxGatewayReasonValid is a reason that is used with the &ldquo;Valid&rdquo; condition when the condition is True.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxGatewayConditionType">NginxGatewayConditionType
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxGatewayConditionType" title="Permanent link">¶</a>
</h3>
<p>
<p>NginxGatewayConditionType is a type of condition associated with an
NginxGateway. This type should be used with the NginxGatewayStatus.Conditions field.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Valid&#34;</p></td>
<td><p>NginxGatewayConditionValid is a condition that is true when the NginxGateway
configuration is syntactically and semantically valid.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxGatewaySpec">NginxGatewaySpec
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxGatewaySpec" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxGateway">NginxGateway</a>)
</p>
<p>
<p>NginxGatewaySpec defines the desired state of the NginxGateway.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>logging</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Logging">
Logging
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Logging defines logging related settings for the control plane.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxGatewayStatus">NginxGatewayStatus
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxGatewayStatus" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxGateway">NginxGateway</a>)
</p>
<p>
<p>NginxGatewayStatus defines the state of the NginxGateway.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.NginxProxySpec">NginxProxySpec
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.NginxProxySpec" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxProxy">NginxProxy</a>)
</p>
<p>
<p>NginxProxySpec defines the desired state of the NginxProxy.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ipFamily</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.IPFamilyType">
IPFamilyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamily specifies the IP family to be used by the NGINX.
Default is &ldquo;dual&rdquo;, meaning the server will use both IPv4 and IPv6.</p>
</td>
</tr>
<tr>
<td>
<code>telemetry</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Telemetry">
Telemetry
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Telemetry specifies the OpenTelemetry configuration.</p>
</td>
</tr>
<tr>
<td>
<code>rewriteClientIP</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.RewriteClientIP">
RewriteClientIP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RewriteClientIP defines configuration for rewriting the client IP to the original client&rsquo;s IP.</p>
</td>
</tr>
<tr>
<td>
<code>disableHTTP2</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableHTTP2 defines if http2 should be disabled for all servers.
Default is false, meaning http2 will be enabled for all servers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.ObservabilityPolicySpec">ObservabilityPolicySpec
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.ObservabilityPolicySpec" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ObservabilityPolicy">ObservabilityPolicy</a>)
</p>
<p>
<p>ObservabilityPolicySpec defines the desired state of the ObservabilityPolicy.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tracing</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Tracing">
Tracing
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tracing allows for enabling and configuring tracing.</p>
</td>
</tr>
<tr>
<td>
<code>targetRefs</code><br/>
<em>
<a href="https://pkg.go.dev/sigs.k8s.io/gateway-api/apis/v1alpha2#LocalPolicyTargetReference">
[]sigs.k8s.io/gateway-api/apis/v1alpha2.LocalPolicyTargetReference
</a>
</em>
</td>
<td>
<p>TargetRefs identifies the API object(s) to apply the policy to.
Objects must be in the same namespace as the policy.
Support: HTTPRoute, GRPCRoute.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.RewriteClientIP">RewriteClientIP
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.RewriteClientIP" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxProxySpec">NginxProxySpec</a>)
</p>
<p>
<p>RewriteClientIP specifies the configuration for rewriting the client&rsquo;s IP address.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.RewriteClientIPModeType">
RewriteClientIPModeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mode defines how NGINX will rewrite the client&rsquo;s IP address.
There are two possible modes:
- ProxyProtocol: NGINX will rewrite the client&rsquo;s IP using the PROXY protocol header.
- XForwardedFor: NGINX will rewrite the client&rsquo;s IP using the X-Forwarded-For header.
Sets NGINX directive real_ip_header: <a href="https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header">https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header</a></p>
</td>
</tr>
<tr>
<td>
<code>setIPRecursively</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>SetIPRecursively configures whether recursive search is used when selecting the client&rsquo;s address from
the X-Forwarded-For header. It is used in conjunction with TrustedAddresses.
If enabled, NGINX will recurse on the values in X-Forwarded-Header from the end of array
to start of array and select the first untrusted IP.
For example, if X-Forwarded-For is [11.11.11.11, 22.22.22.22, 55.55.55.1],
and TrustedAddresses is set to 55.55.55.<sup>1</sup>&frasl;<sub>32</sub>, NGINX will rewrite the client IP to 22.22.22.22.
If disabled, NGINX will select the IP at the end of the array.
In the previous example, 55.55.55.1 would be selected.
Sets NGINX directive real_ip_recursive: <a href="https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_recursive">https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_recursive</a></p>
</td>
</tr>
<tr>
<td>
<code>trustedAddresses</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Address">
[]Address
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TrustedAddresses specifies the addresses that are trusted to send correct client IP information.
If a request comes from a trusted address, NGINX will rewrite the client IP information,
and forward it to the backend in the X-Forwarded-For* and X-Real-IP headers.
If the request does not come from a trusted address, NGINX will not rewrite the client IP information.
TrustedAddresses only supports CIDR blocks: 192.33.21.<sup>1</sup>&frasl;<sub>24</sub>, fe80::<sup>1</sup>&frasl;<sub>64</sub>.
To trust all addresses (not recommended for production), set to 0.0.0.0/0.
If no addresses are provided, NGINX will not rewrite the client IP information.
Sets NGINX directive set_real_ip_from: <a href="https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from">https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from</a>
This field is required if mode is set.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.RewriteClientIPModeType">RewriteClientIPModeType
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.RewriteClientIPModeType" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.RewriteClientIP">RewriteClientIP</a>)
</p>
<p>
<p>RewriteClientIPModeType defines how NGINX Gateway Fabric will determine the client&rsquo;s original IP address.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ProxyProtocol&#34;</p></td>
<td><p>RewriteClientIPModeProxyProtocol configures NGINX to accept PROXY protocol and
set the client&rsquo;s IP address to the IP address in the PROXY protocol header.
Sets the proxy_protocol parameter on the listen directive of all servers and sets real_ip_header
to proxy_protocol: <a href="https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header">https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header</a>.</p>
</td>
</tr><tr><td><p>&#34;XForwardedFor&#34;</p></td>
<td><p>RewriteClientIPModeXForwardedFor configures NGINX to set the client&rsquo;s IP address to the
IP address in the X-Forwarded-For HTTP header.
<a href="https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header">https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header</a>.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Size">Size
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Size" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ClientBody">ClientBody</a>)
</p>
<p>
<p>Size is a string value representing a size. Size can be specified in bytes, kilobytes (k), megabytes (m),
or gigabytes (g).
Examples: 1024, 8k, 1m.</p>
</p>
<h3 id="gateway.nginx.org/v1alpha1.SpanAttribute">SpanAttribute
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.SpanAttribute" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Telemetry">Telemetry</a>,
<a href="#gateway.nginx.org/v1alpha1.Tracing">Tracing</a>)
</p>
<p>
<p>SpanAttribute is a key value pair to be added to a tracing span.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
<p>Key is the key for a span attribute.
Format: must have all &lsquo;&ldquo;&rsquo; escaped and must not contain any &lsquo;$&rsquo; or end with an unescaped &lsquo;\&rsquo;</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value is the value for a span attribute.
Format: must have all &lsquo;&ldquo;&rsquo; escaped and must not contain any &lsquo;$&rsquo; or end with an unescaped &lsquo;\&rsquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Telemetry">Telemetry
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Telemetry" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.NginxProxySpec">NginxProxySpec</a>)
</p>
<p>
<p>Telemetry specifies the OpenTelemetry configuration.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>exporter</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.TelemetryExporter">
TelemetryExporter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Exporter specifies OpenTelemetry export parameters.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceName is the &ldquo;service.name&rdquo; attribute of the OpenTelemetry resource.
Default is &lsquo;ngf:<gateway-namespace>:<gateway-name>&rsquo;. If a value is provided by the user,
then the default becomes a prefix to that value.</p>
</td>
</tr>
<tr>
<td>
<code>spanAttributes</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.SpanAttribute">
[]SpanAttribute
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SpanAttributes are custom key/value attributes that are added to each span.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.TelemetryExporter">TelemetryExporter
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.TelemetryExporter" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Telemetry">Telemetry</a>)
</p>
<p>
<p>TelemetryExporter specifies OpenTelemetry export parameters.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>interval</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.Duration">
Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Interval is the maximum interval between two exports.
Default: <a href="https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter">https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter</a></p>
</td>
</tr>
<tr>
<td>
<code>batchSize</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>BatchSize is the maximum number of spans to be sent in one batch per worker.
Default: <a href="https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter">https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter</a></p>
</td>
</tr>
<tr>
<td>
<code>batchCount</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>BatchCount is the number of pending batches per worker, spans exceeding the limit are dropped.
Default: <a href="https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter">https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter</a></p>
</td>
</tr>
<tr>
<td>
<code>endpoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>Endpoint is the address of OTLP/gRPC endpoint that will accept telemetry data.
Format: alphanumeric hostname with optional http scheme and optional port.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.TraceContext">TraceContext
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.TraceContext" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Tracing">Tracing</a>)
</p>
<p>
<p>TraceContext specifies how to propagate traceparent/tracestate headers.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;extract&#34;</p></td>
<td><p>TraceContextExtract uses an existing trace context from the request, so that the identifiers
of a trace and the parent span are inherited from the incoming request.</p>
</td>
</tr><tr><td><p>&#34;ignore&#34;</p></td>
<td><p>TraceContextIgnore skips context headers processing.</p>
</td>
</tr><tr><td><p>&#34;inject&#34;</p></td>
<td><p>TraceContextInject adds a new context to the request, overwriting existing headers, if any.</p>
</td>
</tr><tr><td><p>&#34;propagate&#34;</p></td>
<td><p>TraceContextPropagate updates the existing context (combines extract and inject).</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.TraceStrategy">TraceStrategy
(<code>string</code> alias)</p><a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.TraceStrategy" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.Tracing">Tracing</a>)
</p>
<p>
<p>TraceStrategy defines the tracing strategy.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;parent&#34;</p></td>
<td><p>TraceStrategyParent enables tracing and only records spans if the parent span was sampled.</p>
</td>
</tr><tr><td><p>&#34;ratio&#34;</p></td>
<td><p>TraceStrategyRatio enables ratio-based tracing, defaulting to 100% sampling rate.</p>
</td>
</tr></tbody>
</table>
<h3 id="gateway.nginx.org/v1alpha1.Tracing">Tracing
<a class="headerlink" href="#gateway.nginx.org%2fv1alpha1.Tracing" title="Permanent link">¶</a>
</h3>
<p>
(<em>Appears on: </em>
<a href="#gateway.nginx.org/v1alpha1.ObservabilityPolicySpec">ObservabilityPolicySpec</a>)
</p>
<p>
<p>Tracing allows for enabling and configuring OpenTelemetry tracing.</p>
</p>
<table class="table table-bordered table-striped">
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.TraceStrategy">
TraceStrategy
</a>
</em>
</td>
<td>
<p>Strategy defines if tracing is ratio-based or parent-based.</p>
</td>
</tr>
<tr>
<td>
<code>ratio</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ratio is the percentage of traffic that should be sampled. Integer from 0 to 100.
By default, 100% of http requests are traced. Not applicable for parent-based tracing.
If ratio is set to 0, tracing is disabled.</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.TraceContext">
TraceContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Context specifies how to propagate traceparent/tracestate headers.
Default: <a href="https://nginx.org/en/docs/ngx_otel_module.html#otel_trace_context">https://nginx.org/en/docs/ngx_otel_module.html#otel_trace_context</a></p>
</td>
</tr>
<tr>
<td>
<code>spanName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SpanName defines the name of the Otel span. By default is the name of the location for a request.
If specified, applies to all locations that are created for a route.
Format: must have all &lsquo;&ldquo;&rsquo; escaped and must not contain any &lsquo;$&rsquo; or end with an unescaped &lsquo;\&rsquo;
Examples of invalid names: some-$value, quoted-&ldquo;value&rdquo;-name, unescaped</p>
</td>
</tr>
<tr>
<td>
<code>spanAttributes</code><br/>
<em>
<a href="#gateway.nginx.org/v1alpha1.SpanAttribute">
[]SpanAttribute
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SpanAttributes are custom key/value attributes that are added to each span.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
