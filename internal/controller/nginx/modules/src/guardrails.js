import fs from 'fs';
import qs from 'querystring';

const GUARDRAILS_URL_VAR = 'guardrails_url';
const GUARDRAILS_TOKEN_FILE_VAR = 'guardrails_token_file';
const GUARDRAILS_FAIL_MODE_VAR = 'guardrails_fail_mode';
const GUARDRAILS_NEXT_PATH_VAR = 'guardrails_next_path';

const FAIL_MODE_OPEN = 'open';
const FAIL_MODE_CLOSED = 'closed';

const BLOCKED_STATUS = 400;
const BLOCKED_MESSAGE = 'Request blocked by AI guardrails\n';

/**
 * Validates the request against the AI guardrails service.
 * If the request is flagged, returns a 400 response.
 * If the request is cleared or fails open, redirects to the next location (EPP).
 *
 * Expected JSON response from guardrails service:
 * {
 *   "result": {
 *     "outcome": "flagged" | "cleared"
 *   }
 * }
 *
 * @param {NginxHTTPRequest} r - The nginx request object
 */
async function validateRequest(r) {
	const guardrailsURL = r.variables[GUARDRAILS_URL_VAR];
	const tokenFile = r.variables[GUARDRAILS_TOKEN_FILE_VAR];
	const failMode = r.variables[GUARDRAILS_FAIL_MODE_VAR] || FAIL_MODE_OPEN;
	const nextPath = r.variables[GUARDRAILS_NEXT_PATH_VAR];

	if (!guardrailsURL) {
		r.error('Missing required variable: ' + GUARDRAILS_URL_VAR);
		return handleFailure(r, failMode, nextPath, 'missing guardrails URL');
	}

	if (!nextPath) {
		r.error('Missing required variable: ' + GUARDRAILS_NEXT_PATH_VAR);
		return handleFailure(r, failMode, nextPath, 'missing next path');
	}

	try {
		const headers = {
			'Content-Type': 'application/json',
		};

		if (tokenFile) {
			try {
				const token = fs.readFileSync(tokenFile, 'utf8').trim();
				if (token) {
					headers['Authorization'] = 'Bearer ' + token;
				}
			} catch (fileErr) {
				r.error('Failed to read guardrails token file: ' + fileErr);
				return handleFailure(r, failMode, nextPath, 'token file read error');
			}
		}

		// Transform the request body for the guardrails service.
		// The client sends the prompt in the "prompt" field, but guardrails expects it in "input".
		let guardrailsBody;
		try {
			const requestJSON = JSON.parse(r.requestText);
			guardrailsBody = JSON.stringify({ input: requestJSON.prompt || '' });
		} catch (parseErr) {
			r.error('Failed to parse request body as JSON: ' + parseErr);
			return handleFailure(r, failMode, nextPath, 'invalid request JSON');
		}

		r.log('Calling guardrails service at: ' + guardrailsURL);

		const response = await ngx.fetch(guardrailsURL, {
			method: 'POST',
			headers: headers,
			body: guardrailsBody,
		});

		if (response.status !== 200) {
			const body = await response.text();
			r.error(
				'Guardrails service returned non-200 status: ' +
					response.status +
					'; body: ' +
					body,
			);
			return handleFailure(r, failMode, nextPath, 'non-200 response');
		}

		const responseBody = await response.text();
		let responseJSON;
		try {
			responseJSON = JSON.parse(responseBody);
		} catch (parseErr) {
			r.error('Failed to parse guardrails response as JSON: ' + parseErr);
			return handleFailure(r, failMode, nextPath, 'invalid JSON response');
		}

		const outcome = responseJSON?.result?.outcome;

		if (outcome === 'flagged') {
			r.log('Request flagged by AI guardrails');
			r.return(BLOCKED_STATUS, BLOCKED_MESSAGE);
			return;
		}

		if (outcome === 'cleared') {
			r.log('Request cleared by AI guardrails');
			return redirectToNext(r, nextPath);
		}

		// Unknown outcome, treat as potential error
		r.warn('Unknown guardrails outcome: ' + outcome);
		return handleFailure(r, failMode, nextPath, 'unknown outcome');
	} catch (err) {
		r.error('Error calling guardrails service: ' + err);
		return handleFailure(r, failMode, nextPath, 'fetch error');
	}
}

/**
 * Handles failure cases based on the configured fail mode.
 *
 * @param {NginxHTTPRequest} r - The nginx request object
 * @param {string} failMode - The fail mode ("open" or "closed")
 * @param {string} nextPath - The next location path to redirect to
 * @param {string} reason - The reason for failure (for logging)
 */
function handleFailure(r, failMode, nextPath, reason) {
	if (failMode === FAIL_MODE_CLOSED) {
		r.log('Guardrails fail-closed: blocking request due to ' + reason);
		r.return(BLOCKED_STATUS, BLOCKED_MESSAGE);
		return;
	}

	// Default to fail-open
	r.log('Guardrails fail-open: allowing request through despite ' + reason);
	return redirectToNext(r, nextPath);
}

/**
 * Redirects to the next location (EPP), preserving query arguments.
 *
 * @param {NginxHTTPRequest} r - The nginx request object
 * @param {string} nextPath - The path to redirect to
 */
function redirectToNext(r, nextPath) {
	// Preserve query arguments in the internal redirect
	let args = qs.stringify(r.args);
	if (args) {
		args = '?' + args;
	}
	r.internalRedirect(nextPath + args);
}

export default { validateRequest };
