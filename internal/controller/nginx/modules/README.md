# NGINX JavaScript Modules

This directory contains the [njs](http://nginx.org/en/docs/njs/) modules for NGINX Gateway Fabric.

## Prerequisites

We recommend using [nvm](https://github.com/nvm-sh/nvm/blob/master/README.md) to install the following dependencies:

- [Node.js](https://nodejs.org/en/) (version 20)
- [npm](https://docs.npmjs.com/)

If you use nvm, you can switch to the recommended version of Node.js by running:

```shell
nvm use
```

Once you've installed Node.js and npm, run `npm install` in this directory to install the rest of the project's
dependencies.

## Modules

- [httpmatches](./src/httpmatches.js): a location handler for HTTP requests. It redirects requests to an internal
  location block based on the request's headers, arguments, and method.

### Helpful Resources for Module Development

When developing njs modules, it's important to remember that njs is a subset of JavaScript, and its compliance with
ECMAScript is still evolving. Not all JavaScript functionality is available in njs, and njs is not fully compatible with
ECMAScript. The following docs are helpful development resources:

- [HTTP njs module](https://nginx.org/en/docs/http/ngx_http_js_module.html)
- [List of njs properties that are compatible with ECMAScript](http://nginx.org/en/docs/njs/compatibility.html)
- [List of njs properties, methods, and objects that are not compatible with ECMAScript](http://nginx.org/en/docs/njs/reference.html)

**Note**: You must use
the [default export statement](https://developer.mozilla.org/en-US/docs/web/javascript/reference/statements/export) to
export functions in an njs module.

## Unit Tests

This project uses the [Mocha](https://mochajs.org/) test framework and the [Chai](https://www.chaijs.com/) assertion
library to write BDD-style unit tests. Tests for the modules are placed in the `/tests` directory and named
as `<module-name>.test.js`.

To run unit tests against the [httpmatches](./src/httpmatches.js) modules you must:

- Use
  the [default import statement](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/import#importing_defaults)
  to import the module.
- Run mocha with the `--require esm` option.
- Mock the [NGINX HTTP Request Object](http://nginx.org/en/docs/njs/reference.html#http) and pass it to the exported
  function. Not all functions and fields on the HTTP request object need to be mocked, just the ones that are used in
  the module.

### Run Unit Tests

To run the unit tests:

```shell
npm test
```

## Debugging

### Debug Unit Tests

To debug on the command-line:

- Set a breakpoint using
  the [debugger](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/debugger) statement.
- Run the tests with the inspect argument:

```shell
npx mocha inspect -r esm
```

If you are using JetBrains or VSCode for development, you can debug the unit tests in your IDE.

For JetBrains:

- [Create a run/debug configuration for mocha](https://www.jetbrains.com/help/idea/run-debug-configuration-mocha.html).
- Add `--require esm` to the `Extra Mocha Options` field in your run/debug configuration.

For VSCode:

- [Create a debug configuration for mocha](https://dev.to/wakeupmh/debugging-mocha-tests-in-vscode-468a).
- Add `--require esm` to the configuration args.

### Log Statements

You can add log statements to debug njs code at runtime. The following log functions are available on
the [NGINX HTTP Request Object](http://nginx.org/en/docs/njs/reference.html#http):

Log at error level:

```shell
r.error(string)
```

Log at info level:

```shell
r.log(string)
```

Log at warn level:

```shell
r.warn(string)
```

## Format Code

This project uses [prettier](https://prettier.io/) to lint and format the JavaScript code. To format the code run:

```shell
npm run format
```
