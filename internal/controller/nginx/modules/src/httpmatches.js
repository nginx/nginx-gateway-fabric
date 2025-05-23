import qs from 'querystring';

const MATCHES_KEY = 'match_key';
const HTTP_CODES = {
	notFound: 404,
	internalServerError: 500,
};

function redirect(r) {
	let matchList;
	try {
		matchList = extractMatchesFromRequest(r, matches);
	} catch (e) {
		r.error(e.message);
		r.return(HTTP_CODES.internalServerError);
		return;
	}
	redirectForMatchList(r, matchList);
}

function extractMatchesFromRequest(r, matches) {
	let matchList;
	if (!r.variables[MATCHES_KEY]) {
		throw Error(
			`cannot redirect the request; the ${MATCHES_KEY} is not defined on the request object`,
		);
	}

	let key = r.variables[MATCHES_KEY];
	if (!matches[key]) {
		throw Error(
			`cannot redirect the request; the key ${key} is not defined on the matches object`,
		);
	}

	// matchList is a list of http matches in order of precedence.
	// We will accept the first match that the request satisfies.
	// If there's a match, redirect request to internal location block.
	// If an exception occurs, return 500.
	// If no matches are found, return 404.
	matchList = matches[key];
	try {
		verifyMatchList(matchList);
	} catch (e) {
		throw Error(`cannot redirect the request; ${matchList} matches list is not valid: ${e}`);
	}

	return matchList;
}

function redirectForMatchList(r, matchList) {
	let match;
	try {
		match = findWinningMatch(r, matchList);
	} catch (e) {
		r.error(e.message);
		r.return(HTTP_CODES.internalServerError);
		return;
	}

	if (!match) {
		r.return(HTTP_CODES.notFound);
		return;
	}

	if (!match.redirectPath) {
		r.error(
			`cannot redirect the request; the match ${JSON.stringify(
				match,
			)} does not have a redirectPath set`,
		);
		r.return(HTTP_CODES.internalServerError);
		return;
	}

	// If performing a rewrite, $request_uri won't be used,
	// so we have to preserve args in the internal redirect.
	let args = qs.stringify(r.args);
	if (args) {
		args = '?' + args;
	}

	r.internalRedirect(match.redirectPath + args);
}

function verifyMatchList(matchList) {
	if (!Array.isArray(matchList)) {
		throw Error(`cannot redirect the request; expected a list of matches, got ${matchList}`);
	}

	if (matchList.length === 0) {
		throw Error(`cannot redirect the request; matches is an empty list`);
	}

	return;
}

function findWinningMatch(r, matches) {
	for (let i = 0; i < matches.length; i++) {
		try {
			let found = testMatch(r, matches[i]);
			if (found) {
				return matches[i];
			}
		} catch (e) {
			throw e;
		}
	}

	return null;
}

function testMatch(r, match) {
	// check for any
	if (match.any) {
		return true;
	}

	// check method
	if (match.method && r.method !== match.method) {
		return false;
	}

	// check headers
	if (match.headers) {
		try {
			let found = headersMatch(r.headersIn, match.headers);
			if (!found) {
				return false;
			}
		} catch (e) {
			throw e;
		}
	}

	// check params
	if (match.params) {
		try {
			let found = paramsMatch(r.args, match.params);
			if (!found) {
				return false;
			}
		} catch (e) {
			throw e;
		}
	}

	// all match conditions are satisfied so return true
	return true;
}

function headersMatch(requestHeaders, headers) {
	for (let i = 0; i < headers.length; i++) {
		const h = headers[i];
		const kv = h.split(':');

		// header should be of the format "key:MatchType:value"
		if (kv.length !== 3) {
			throw Error(`invalid header match: ${h}`);
		}
		// Header names are compared in a case-insensitive manner, meaning header name "FOO" is equivalent to "foo".
		// The NGINX request's headersIn object lookup is case-insensitive as well.
		// This means that requestHeaders['FOO'] is equivalent to requestHeaders['foo'].
		let val = requestHeaders[kv[0]];

		if (!val) {
			return false;
		}

		// split on comma because nginx uses commas to delimit multiple header values
		const values = val.split(',');

		let type = kv[1];
		// verify the type of header match
		if (!(type == 'Exact' || type == 'RegularExpression')) {
			throw Error(`invalid header match type: ${type}`);
		}

		// match the value based on the type
		if (type === 'Exact') {
			if (!values.includes(kv[2])) {
				return false;
			}
		} else if (type === 'RegularExpression') {
			if (!values.some((v) => new RegExp(kv[2]).test(v))) {
				return false;
			}
		}
	}

	return true;
}

function paramsMatch(requestParams, params) {
	for (let i = 0; i < params.length; i++) {
		let p = params[i];
		// We store query parameter matches as strings with the format "key=MatchType=value"; however, there may be more than one
		// instance of "=" in the string.
		// To recover the key, type and and value, we need to find the first occurrence of "=" in the string.
		const firstIdx = p.indexOf('=');

		// Check for an improperly constructed query parameter match. There are two possible error cases:
		// (1) if the index is -1, then there are no "=" in the string (e.g. "keyExactvalue")
		// (2) if the index is 0, then there is no key in the string (e.g. "=Exact=value").
		if (firstIdx === -1 || firstIdx === 0) {
			throw Error(`invalid query parameter: ${p}`);
		}

		// find the next occurence of "=" in the string
		const idx = p.indexOf('=', firstIdx + 1);

		// Three possible error cases for improperly constructed query parameter match:
		// (1) if the index is -1, then there are no second occurence of "=" in the string (e.g. "key=Exactvalue")
		// (2) if the index is 0, then there is no value in the string and has only one "=" (e.g. "key=Exact").
		// (3) if the index is equal to length -1, then there is no key and type in the string (e.g. "=value").
		if (idx === -1 || idx === 0 || idx === p.length - 1) {
			throw Error(`invalid query parameter: ${p}`);
		}

		// extract the type match from the string
		const type = p.slice(firstIdx + 1, idx);

		if (!(type == 'Exact' || type == 'RegularExpression')) {
			throw Error(`invalid header match type: ${type}`);
		}

		// Divide string into key value using the index.
		let kv = [p.slice(0, firstIdx), p.slice(idx + 1)];

		// val can either be a string or an array of strings.
		// Also, the NGINX request's args object lookup is case-sensitive.
		// For example, 'a=1&b=2&A=3&b=4' will be parsed into {a: "1", b: ["2", "4"], A: "3"}
		let val = requestParams[kv[0]];
		if (!val) {
			return false;
		}

		// If val is an array, we will match against the first element in the array according to the Gateway API spec.
		if (Array.isArray(val)) {
			val = val[0];
		}

		// match the value based on the type
		if (type === 'Exact') {
			if (val !== kv[1]) {
				return false;
			}
		} else if (type === 'RegularExpression') {
			if (!new RegExp(kv[1]).test(val)) {
				return false;
			}
		}
	}

	return true;
}

export default {
	redirect,
	redirectForMatchList,
	extractMatchesFromRequest,
	MATCHES_KEY,
	verifyMatchList,
	testMatch,
	findWinningMatch,
	headersMatch,
	paramsMatch,
	HTTP_CODES,
};
