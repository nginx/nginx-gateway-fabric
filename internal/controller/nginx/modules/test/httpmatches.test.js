import { default as hm } from '../src/httpmatches.js';
import { describe, expect, it } from 'vitest';

// Creates a NGINX HTTP Request Object for testing.
// See documentation for all properties available: http://nginx.org/en/docs/njs/reference.html
function createRequest({ method = '', headers = {}, params = {}, matchKey = '' } = {}) {
	let r = {
		// Test mocks
		return(statusCode) {
			r.testReturned = statusCode;
		},
		internalRedirect(redirectPath) {
			r.testRedirectedTo = redirectPath;
		},
		error(msg) {
			console.log('\tngx_error:', msg);
		},
		variables: {},
	};

	if (method) {
		r.method = method;
	}

	if (headers) {
		r.headersIn = headers;
	}

	if (params) {
		r.args = params;
	}

	if (matchKey) {
		r.variables[hm.MATCHES_KEY] = matchKey;
	}

	return r;
}

describe('extractMatchesFromRequest', () => {
	const tests = [
		{
			name: 'throws if match_key variable does not exist on request',
			request: createRequest({}),
			matchesObject: {},
			expectThrow: true,
			errSubstring: 'match_key is not defined',
		},
		{
			name: 'throws if key does not exist on matches object',
			request: createRequest({ matchKey: 'test' }),
			matchesObject: {},
			expectThrow: true,
			errSubstring: 'the key test is not defined on the matches object',
		},
		{
			name: 'throws an error if matchList is not valid',
			request: createRequest({ matchKey: 'test' }),
			matchesObject: { test: {} },
			expectThrow: true,
			errSubstring:
				'matches list is not valid: Error: cannot redirect the request; expected a list of matches',
		},
		{
			name: 'does not throw if matches key is present & expected matchList is returned',
			request: createRequest({ matchKey: 'test' }),
			matchesObject: { test: [{ match: 'value' }] },
			expectThrow: false,
			expected: [{ match: 'value' }],
		},
	];
	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() =>
					hm.extractMatchesFromRequest(test.request, test.matchesObject),
				).to.throw(test.errSubstring);
			} else {
				expect(
					hm.extractMatchesFromRequest(test.request, test.matchesObject),
				).to.deep.equal(test.expected);
			}
		});
	});
});

describe('verifyMatchList', () => {
	const tests = [
		{
			name: 'throws if matches variable is not an array',
			matchList: '{}',
			expectThrow: true,
			errSubstring: 'expected a list of matches',
		},
		{
			name: 'throws if the length of the matches variable is zero',
			matchList: [],
			expectThrow: true,
			errSubstring: 'matches is an empty list',
		},
		{
			name: 'does not throw if matches variable is expected list of matches',
			matchList: '[{"any":true}]',
			expectThrow: false,
		},
	];
	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() => hm.verifyMatchList(test.matchList)).to.throw(test.errSubstring);
			} else {
				expect(() => hm.verifyMatchList(test.matchList).to.not.throw());
			}
		});
	});
});

describe('testMatch', () => {
	const tests = [
		{
			name: 'returns true if any is set to true',
			match: { any: true },
			request: createRequest(),
			expected: true,
		},
		{
			name: 'returns true if method matches and no other conditions are set',
			match: { method: 'GET' },
			request: createRequest({ method: 'GET' }),
			expected: true,
		},
		{
			name: 'returns true if headers match and no other conditions are set',
			match: { headers: ['header:Exact:value'] },
			request: createRequest({ headers: { header: 'value' } }),
			expected: true,
		},
		{
			name: 'returns true if query parameters match and no other conditions are set',
			match: { params: ['key=Exact=value'] },
			request: createRequest({ params: { key: 'value' } }),
			expected: true,
		},
		{
			name: 'returns true if multiple conditions match',
			match: { method: 'GET', headers: ['header:Exact:value'], params: ['key=Exact=value'] },
			request: createRequest({
				method: 'GET',
				headers: { header: 'value' },
				params: { key: 'value' },
			}),
			expected: true,
		},
		{
			name: 'returns false if method does not match',
			match: { method: 'POST' },
			request: createRequest({ method: 'GET' }),
			expected: false,
		},
		{
			name: 'returns false if headers do not match',
			match: { method: 'GET', headers: ['header:Exact:value'] },
			request: createRequest({ method: 'GET' }), // no headers are set on request
			expected: false,
		},
		{
			name: 'returns false if query parameters do not match',
			match: { method: 'GET', headers: ['header:Exact:value'], params: ['key=Exact=value'] },
			request: createRequest({ method: 'GET', headers: { header: 'value' } }), // no params set on request
			expected: false,
		},
		{
			name: 'throws if headers are malformed',
			match: { headers: ['malformedheader'] },
			request: createRequest(),
			expectThrow: true,
			errSubstring: 'invalid header match',
		},
		{
			name: 'throws if params are malformed',
			match: { params: ['keyvalue'] },
			request: createRequest(),
			expectThrow: true,
			errSubstring: 'invalid query parameter',
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() => hm.testMatch(test.request, test.match)).to.throw(test.errSubstring);
			} else {
				const result = hm.testMatch(test.request, test.match);
				expect(result).to.equal(test.expected);
			}
		});
	});
});

describe('findWinningMatch', () => {
	const headerMatch = { headers: ['header:Exact:value'] };
	const queryParamMatch = { params: ['key=Exact=value'] };
	const methodMatch = { method: 'POST' };
	const anyMatch = { any: true };
	const malformedMatch = { headers: ['malformed'] };

	const tests = [
		{
			name: 'returns first match that the request satisfies',
			matches: [headerMatch, queryParamMatch, methodMatch, anyMatch], // second match should be returned
			request: createRequest({ method: 'POST', params: { key: 'value' } }),
			expected: queryParamMatch,
		},
		{
			name: 'returns null when no match exists',
			matches: [headerMatch, queryParamMatch, methodMatch],
			request: createRequest({ method: 'GET' }),
			expected: null,
		},
		{
			name: 'throws if an exception occurs while finding a match',
			matches: [headerMatch, queryParamMatch, malformedMatch],
			request: createRequest({ method: 'GET' }),
			expectThrow: true,
			errSubstring: 'invalid header match',
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() => hm.findWinningMatch(test.request, test.matches)).to.throw(
					test.errSubstring,
				);
			} else {
				const result = hm.findWinningMatch(test.request, test.matches);
				expect(result).to.equal(test.expected);
			}
		});
	});
});

describe('headersMatch', () => {
	const multipleHeaders = [
		'header1:Exact:VALUE1',
		'header2:RegularExpression:Header-[a-z]{1}',
		'header3:Exact:value3',
	]; // case matters for header values

	const tests = [
		{
			name: 'throws an error if a header has multiple colons',
			headers: ['too:Exact:many:colons'],
			expectThrow: true,
		},
		{
			name: 'throws an error if a header has no colon',
			headers: ['wrong=delimiter'],
			requestHeaders: {},
			expectThrow: true,
		},
		{
			name: 'throws an error if a header has invalid match type',
			headers: ['key:Incorrect:val'],
			requestHeaders: {
				key: 'val',
			},
			expectTypeError: true,
		},
		{
			name: 'returns false if one of the header values does not match',
			headers: multipleHeaders,
			requestHeaders: {
				header1: 'VALUE1',
				header2: 'value2',
				header3: 'wrong-value', // this value does not match
			},
			expected: false,
		},
		{
			name: 'returns false if one of the header values case does not match',
			headers: multipleHeaders,
			requestHeaders: {
				header1: 'value1', // this value is not the correct case
				header2: 'value2',
				header3: 'value3',
			},
			expected: false,
		},
		{
			name: 'returns true if all headers match',
			headers: multipleHeaders,
			requestHeaders: {
				header1: 'VALUE1', // this value is not the correct case
				header2: 'Header-a',
				header3: 'value3',
			},
			expected: true,
		},
		{
			name: 'returns true if request has multiple values for a header name and one value matches ',
			headers: ['multiValueHeader:Exact:val3'],
			requestHeaders: {
				multiValueHeader: 'val1,val2,val3,val4,val5',
			},
			expected: true,
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() => hm.headersMatch(test.requestHeaders, test.headers)).to.throw(
					'invalid header match',
				);
			} else if (test.expectTypeError) {
				expect(() => hm.headersMatch(test.requestHeaders, test.headers)).to.throw(
					'invalid header match type',
				);
			} else {
				expect(hm.headersMatch(test.requestHeaders, test.headers)).to.equal(test.expected);
			}
		});
	});
});

describe('paramsMatch', () => {
	const params = [
		'Arg1=Exact=value1',
		'arg2=Exact=value2=SOME=other=value',
		'arg3=Exact===value3&*1(*+',
	]; // case matters for header values

	const tests = [
		{
			name: 'throws an error a param has no key',
			params: ['=Exact=nokey'],
			expectThrow: true,
		},
		{
			name: 'throws an error if a param has no value',
			params: ['novalue=Exact='],
			expectThrow: true,
		},
		{
			name: 'throws an error a param has no equal sign delimiter',
			params: ['keyval'],
			expectThrow: true,
		},
		{
			name: 'throws an error if a param has invalid match type',
			params: ['key=Incorrect=val'],
			expectTypeError: true,
		},
		{
			name: 'returns false if one of the params is missing from request',
			params: params,
			requestParams: {
				// Arg1 is missing,
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: false,
		},
		{
			name: 'returns false if one of the params has an empty value',
			params: params,
			requestParams: {
				Arg1: 'value1',
				arg2: 'value2=SOME=other=value',
				arg3: '', // empty value
			},
			expected: false,
		},
		{
			name: 'returns false if one of the param values does not match',
			params: params,
			requestParams: {
				Arg1: 'Arg1=not-value-1', // this value does not match
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: false,
		},
		{
			name: 'returns false if the case of one param values does not match',
			params: params,
			requestParams: {
				Arg1: 'VALUE1', // this value is not the correct case
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: false,
		},
		{
			name: 'returns false if the case of one param name does not match',
			params: params,
			requestParams: {
				Arg1: 'value1',
				arg2: 'value2=SOME=other=value',
				ARG3: '==value3&*1(*+', // this param name is not the correct case
			},
			expected: false,
		},
		{
			name: 'returns true if all params match',
			params: params,
			requestParams: {
				Arg1: 'value1',
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: true,
		},
		{
			name: 'returns true if all params match with one param having multiple values',
			params: params,
			requestParams: {
				Arg1: ['value1', 'value2'], // 'value1' wins
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: true,
		},
		{
			name: 'returns true if param matches the regular expression',
			params: ['key=RegularExpression=Query-[a-z]{1}'],
			requestParams: {
				key: 'Query-a',
			},
			expected: true,
		},
		{
			name: 'returns false if param does not match the regular expression',
			params: ['key=RegularExpression=Query-[a-z]{1}'],
			requestParams: {
				key: 'value',
			},
			expected: false,
		},
		{
			name: 'returns false if one param does not match because of multiple values',
			params: params,
			requestParams: {
				Arg1: ['value2', 'value1'], // 'value2' wins but it does not match
				arg2: 'value2=SOME=other=value',
				arg3: '==value3&*1(*+',
			},
			expected: false,
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			if (test.expectThrow) {
				expect(() => hm.paramsMatch(test.requestParams, test.params)).to.throw(
					'invalid query parameter',
				);
			} else if (test.expectTypeError) {
				expect(() => hm.paramsMatch(test.requestParams, test.params)).to.throw(
					'invalid header match type',
				);
			} else {
				expect(hm.paramsMatch(test.requestParams, test.params)).to.equal(test.expected);
			}
		});
	});
});

describe('redirectForMatchList', () => {
	const testAnyMatch = { any: true, redirectPath: '/any' };
	const testHeaderMatches = {
		headers: ['header1:Exact:VALUE1', 'header2:Exact:value2', 'header3:Exact:value3'],
		redirectPath: '/headers',
	};
	const testQueryParamMatches = {
		params: [
			'Arg1=Exact=value1',
			'arg2=Exact=value2=SOME=other=value',
			'arg3=Exact===value3&*1(*+',
		],
		redirectPath: '/params',
	};
	const testAllMatchTypes = {
		method: 'GET',
		headers: ['header1:Exact:value1', 'header2:Exact:value2'],
		params: ['Arg1=Exact=value1', 'arg2=Exact=value2=SOME=other=value'],
		redirectPath: '/a-match',
	};

	const tests = [
		{
			name: 'returns Internal Server Error status code if match variable is not set',
			request: createRequest(),
			matches: null,
			expectedReturn: hm.HTTP_CODES.internalServerError,
		},
		{
			name: 'returns Internal Server Error status code if matchList contains malformed match',
			request: createRequest(),
			matches: [{ headers: ['malformedheader'] }],
			expectedReturn: hm.HTTP_CODES.internalServerError,
		},
		{
			name: 'returns Not Found status code if request does not satisfy any match',
			request: createRequest({ method: 'GET' }),
			matches: [{ method: 'POST' }],
			expectedReturn: hm.HTTP_CODES.notFound,
		},
		{
			name: 'returns Internal Server Error status code if request satisfies match, but the redirectPath is missing',
			request: createRequest({ method: 'GET' }),
			matches: [{ method: 'GET' }],
			expectedReturn: hm.HTTP_CODES.internalServerError,
		},
		{
			name: 'redirects to the redirectPath of the first match the request satisfies',
			request: createRequest({
				method: 'GET',
				headers: { header1: 'value1', header2: 'value2' },
				params: { Arg1: 'value1', arg2: 'value2=SOME=other=value' },
			}),
			matches: [testHeaderMatches, testQueryParamMatches, testAllMatchTypes, testAnyMatch], // request matches testAllMatchTypes and testAnyMatch. But first match should win.
			expectedRedirect: '/a-match?Arg1=value1&arg2=value2%3DSOME%3Dother%3Dvalue',
		},
	];

	tests.forEach((test) => {
		it(test.name, () => {
			hm.redirectForMatchList(test.request, test.matches);
			if (test.expectedReturn) {
				expect(test.request.testReturned).to.equal(test.expectedReturn);
			} else if (test.expectedRedirect) {
				expect(test.request.testRedirectedTo).to.equal(test.expectedRedirect);
			}
		});
	});
});
