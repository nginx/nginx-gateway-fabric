# NJS Modules

This directory contains [NJS](https://nginx.org/en/docs/njs/) (NGINX JavaScript) modules for NGINX Gateway Fabric.

## Directory Structure

```text
njs/
├── src/                     # NJS source files
│   └── httpmatches.js       # HTTP request matching module
├── test/                    # Unit tests
│   ├── httpmatches.test.js
│   └── vitest.config.ts
├── .prettierrc              # Prettier configuration
├── package.json
└── package-lock.json
```

## Modules

### httpmatches

An NJS location handler for HTTP requests. It redirects requests to internal location blocks based on the request's
headers, arguments, and method.

## Development

### Prerequisites

We recommend using [nvm](https://github.com/nvm-sh/nvm/blob/master/README.md) to manage Node.js versions.

**Requirements:**

- [Node.js](https://nodejs.org/en/) (version 20)
- [npm](https://docs.npmjs.com/)

If you use nvm, switch to the recommended version by running:

```shell
nvm use
```

Then install dependencies:

```shell
npm install
```

### Helpful Resources

NJS is a subset of JavaScript with evolving ECMAScript compliance. Not all JavaScript functionality is available.
These resources are helpful for development:

- [HTTP njs module](https://nginx.org/en/docs/http/ngx_http_js_module.html)
- [NJS ECMAScript compatibility](http://nginx.org/en/docs/njs/compatibility.html)
- [NJS reference (non-ECMAScript features)](http://nginx.org/en/docs/njs/reference.html)

> **Note:** You must use the
> [default export statement](https://developer.mozilla.org/en-US/docs/web/javascript/reference/statements/export)
> to export functions in an NJS module.

### Unit Tests

This project uses [vitest](https://vitest.dev/) for testing. Tests are in the `test/` directory and named
`<module-name>.test.js`.

To test the [httpmatches](./src/httpmatches.js) module:

- Use the [default import](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/import#importing_defaults)
  to import the module
- Mock the [NGINX HTTP Request Object](http://nginx.org/en/docs/njs/reference.html#http) (only mock fields used by the module)

Run tests:

```shell
npm test
```

Or from the repository root:

```shell
make njs-unit-test
```

### Format Code

This project uses [prettier](https://prettier.io/) for code formatting.

```shell
npm run format
```

Or from the repository root:

```shell
make njs-fmt
```

### Debugging

Add log statements to debug NJS code at runtime using the
[NGINX HTTP Request Object](http://nginx.org/en/docs/njs/reference.html#http):

```javascript
// Error level
r.error("message");

// Info level
r.log("message");

// Warn level
r.warn("message");
```
