import { default as epp } from '../src/epp.js';
import { expect, describe, it } from 'vitest';

function makeRequest(body) {
	let r = {
		// Test mocks
		error(msg) {
			r.variables.error = msg;
		},
		requestText: body,
		variables: {},
	};

	return r;
}

describe('extractModel', () => {
	const tests = [
		{
			name: 'returns the model value',
			body: '{"model":"gpt-4"}',
			model: 'gpt-4',
			error: undefined,
		},
		{
			name: 'returns empty string if model is missing',
			body: '{"foo":1}',
			model: '',
			error: 'request body does not contain model parameter',
		},
		{
			name: 'returns empty string for invalid JSON',
			body: 'not-json',
			model: '',
			error: `error parsing request body for model name: Unexpected token 'o', "not-json" is not valid JSON`,
		},
		{
			name: 'empty request body',
			body: '',
			model: '',
			error: 'error parsing request body for model name: Unexpected end of JSON input',
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			let r = makeRequest(test.body);
			expect(epp.extractModel(r)).to.equal(test.model);
			expect(r.variables.error).to.equal(test.error);
		});
	});
});
