import qs from 'querystring';

const EPP_HOST_HEADER_VAR = 'epp_host';
const EPP_PORT_HEADER_VAR = 'epp_port';
const EPP_HOST_HEADER = 'X-EPP-Host';
const EPP_PORT_HEADER = 'X-EPP-Port';
const ENDPOINT_HEADER = 'X-Gateway-Destination-Endpoint';
const EPP_INTERNAL_PATH_VAR = 'epp_internal_path';
const WORKLOAD_ENDPOINT_VAR = 'inference_workload_endpoint';
const SHIM_URI = 'http://127.0.0.1:54800';

async function getEndpoint(r) {
	if (!r.variables[EPP_HOST_HEADER_VAR] || !r.variables[EPP_PORT_HEADER_VAR]) {
		throw Error(
			`Missing required variables: ${EPP_HOST_HEADER_VAR} and/or ${EPP_PORT_HEADER_VAR}`,
		);
	}
	if (!r.variables[EPP_INTERNAL_PATH_VAR]) {
		throw Error(`Missing required variable: ${EPP_INTERNAL_PATH_VAR}`);
	}

	let headers = Object.assign({}, r.headersIn);
	headers[EPP_HOST_HEADER] = r.variables[EPP_HOST_HEADER_VAR];
	headers[EPP_PORT_HEADER] = r.variables[EPP_PORT_HEADER_VAR];

	try {
		const response = await ngx.fetch(SHIM_URI, {
			method: r.method,
			headers: headers,
			body: r.requestText,
		});
		const endpointHeader = response.headers.get(ENDPOINT_HEADER);
		if (response.status === 200 && endpointHeader) {
			r.variables[WORKLOAD_ENDPOINT_VAR] = endpointHeader;
			r.log(
				`found inference endpoint from EndpointPicker: ${r.variables[WORKLOAD_ENDPOINT_VAR]}`,
			);
		} else {
			const body = await response.text();
			r.error(
				`could not get specific inference endpoint from EndpointPicker; ` +
					`status: ${response.status}; body: ${body}`,
			);
		}
	} catch (err) {
		r.error(`Error in ngx.fetch: ${err}`);
	}

	// If performing a rewrite, $request_uri won't be used,
	// so we have to preserve args in the internal redirect.
	let args = qs.stringify(r.args);
	if (args) {
		args = '?' + args;
	}

	r.internalRedirect(r.variables[EPP_INTERNAL_PATH_VAR] + args);
}

export default { getEndpoint };
