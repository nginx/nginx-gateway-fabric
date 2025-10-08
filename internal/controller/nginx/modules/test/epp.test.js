import { default as epp } from '../src/epp.js';
import { expect, describe, it, beforeEach, afterEach, vi } from 'vitest';

function makeRequest({
	method = 'POST',
	headersIn = {},
	args = {},
	requestText = '',
	variables = {},
} = {}) {
	return {
		method,
		headersIn,
		requestText,
		variables,
		args,
		error: vi.fn(),
		log: vi.fn(),
		internalRedirect: vi.fn(),
	};
}

describe('getEndpoint', () => {
	let originalNgx;
	beforeEach(() => {
		originalNgx = globalThis.ngx;
	});
	afterEach(() => {
		globalThis.ngx = originalNgx;
	});

	it('throws if host or port is missing', async () => {
		const r = makeRequest({ variables: { epp_internal_path: '/foo' } });
		await expect(epp.getEndpoint(r)).rejects.toThrow(/Missing required variables/);
	});

	it('throws if internal path is missing', async () => {
		const r = makeRequest({ variables: { epp_host: 'host', epp_port: '1234' } });
		await expect(epp.getEndpoint(r)).rejects.toThrow(/Missing required variable/);
	});

	it('sets endpoint and logs on 200 with endpoint header', async () => {
		const endpoint = 'http://endpoint';
		globalThis.ngx = {
			fetch: vi.fn().mockResolvedValue({
				status: 200,
				headers: { get: () => endpoint },
				text: vi.fn(),
			}),
		};
		const r = makeRequest({
			variables: { epp_host: 'host', epp_port: '1234', epp_internal_path: '/foo' },
		});
		await epp.getEndpoint(r);
		expect(r.variables.inference_workload_endpoint).toBe(endpoint);
		expect(r.log).toHaveBeenCalledWith(expect.stringContaining(endpoint));
		expect(r.internalRedirect).toHaveBeenCalledWith('/foo');
	});

	it('calls error if response is not 200 or endpoint header missing', async () => {
		globalThis.ngx = {
			fetch: vi.fn().mockResolvedValue({
				status: 404,
				headers: { get: () => null },
				text: vi.fn().mockResolvedValue('fail'),
			}),
		};
		const r = makeRequest({
			variables: { epp_host: 'host', epp_port: '1234', epp_internal_path: '/foo' },
		});
		await epp.getEndpoint(r);
		expect(r.error).toHaveBeenCalledWith(
			expect.stringContaining('could not get specific inference endpoint'),
		);
		expect(r.internalRedirect).toHaveBeenCalledWith('/foo');
	});

	it('calls error if fetch throws', async () => {
		globalThis.ngx = {
			fetch: vi.fn().mockRejectedValue(new Error('network fail')),
		};
		const r = makeRequest({
			variables: { epp_host: 'host', epp_port: '1234', epp_internal_path: '/foo' },
		});
		await epp.getEndpoint(r);
		expect(r.error).toHaveBeenCalledWith(expect.stringContaining('Error in ngx.fetch'));
		expect(r.internalRedirect).toHaveBeenCalledWith('/foo');
	});

	it('preserves args in internal redirect when args are present', async () => {
		const endpoint = 'http://endpoint';
		globalThis.ngx = {
			fetch: vi.fn().mockResolvedValue({
				status: 200,
				headers: { get: () => endpoint },
				text: vi.fn(),
			}),
		};
		const r = makeRequest({
			variables: { epp_host: 'host', epp_port: '1234', epp_internal_path: '/foo' },
			args: { a: '1', b: '2' },
		});
		await epp.getEndpoint(r);
		expect(r.internalRedirect).toHaveBeenCalledWith('/foo?a=1&b=2');
	});
});