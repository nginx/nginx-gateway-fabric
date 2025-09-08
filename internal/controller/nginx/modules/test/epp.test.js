import { default as epp } from '../src/epp.js';
import { expect, describe, it } from 'vitest';

function makeRequest(body) {
	let r = {
		// Test mocks
		error(msg) {
			console.log('\tngx_error:', msg);
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
		},
		{
			name: 'returns empty string if model is missing',
			body: '{"foo":1}',
			model: '',
		},
		{
			name: 'returns empty string for invalid JSON',
			body: 'not-json',
			model: '',
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			expect(epp.extractModel(makeRequest(test.body))).to.equal(test.model);
		});
	});
});
