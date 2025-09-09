// This file contains the methods to get an AI workload endpoint from the EndpointPicker (EPP).

// TODO (sberman): this module will need to be enhanced to include the following:
// - function that sends the subrequest to the Go middleware application (to get the endpoint from EPP)
// - if a user has specified an Exact matching condition for a model name, extract the model name from
// the request body, and if it matches that condition, set the proper value in the X-Gateway-Model-Name header
// (based on if we do a redirect or traffic split (see design doc)) in the subrequest. If the client request
// already has this header set, then I don't think we need to extract the model from the body, just pass
// through the existing header.
// I believe we have to use js_content to call the NJS functionality. Because this takes over
// the request, we will likely have to finish the NJS functionality with an internalRedirect to an internal
// location that proxy_passes to the chosen endpoint.

// extractModel extracts the model name from the request body.
function extractModel(r) {
	try {
		var body = JSON.parse(r.requestText);
		if (body && body.model !== undefined) {
			return String(body.model);
		}
	} catch (e) {
		r.error(`error parsing request body for model name: ${e.message}`);
		return '';
	}
	r.error('request body does not contain model parameter');
	return '';
}

export default { extractModel };
