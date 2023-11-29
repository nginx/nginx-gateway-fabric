# NGINX Gateway Fabric Docs

The `/site` directory contains the user documentation for NGINX Gateway Fabric and the requirements for linting, building, and publishing the docs. Run all the `hugo` commands below from this directory.

We use [Hugo](https://gohugo.io/) to build the docs for NGINX, with the [nginx-hugo-theme](https://github.com/nginxinc/nginx-hugo-theme).

Docs should be written in Markdown.

In the `/site` directory, you will find the following files:

- a [Netlify](https://netlify.com) configuration file;
- configuration files for [markdownlint](https://github.com/DavidAnson/markdownlint/) and [markdown-link-check](https://github.com/tcort/markdown-link-check)
- a `./config` directory that contains the [Hugo](https://gohugo.io) configuration.

## Git Guidelines

See the [Pull Request Guide](pull-request.md) for specfic instructions on how to submit a pull request.

### Branching and Workflow

This repo uses a [forking workflow](https://www.atlassian.com/git/tutorials/comparing-workflows/forking-workflow). See our [Branching and Workflow](branching-and-workflow.md) documentation for more information.

### Publishing Documentation Updates

**`main`** is the default branch in this repo. All the latest content updates are merged into this branch.

The documentation is published from the latest public release branch, (for example, `release-4.0`). Work on your docs in a feature branch in your fork of the repo. Open pull requests into the `main` branch when you are ready to merge your work.

If you are working on content for immediate publication in the docs site, cherrypick your changes to the current public release branch.

If you are working on content for a future release, make sure that you **do not** cherrypick them to the current public release branch, as this will publish them automatically. See the [Release Process documentation](release-process.md) for more information.


## Setup

### Golang

Follow the instructions here to install Go: https://golang.org/doc/install

> To support the use of Hugo mods, you need to install Go v1.15 or newer.

### Hugo

Follow the instructions here to install Hugo: [Hugo Installation](https://gohugo.io/installation/)

> **NOTE:** We are currently running [Hugo v0.115.3](https://github.com/gohugoio/hugo/releases/tag/v0.115.3) in production.

### Markdownlint

We use markdownlint to check that Markdown files are correctly formatted. You can use `npm` to install markdownlint-cli:

```shell
npm install -g markdownlint-cli
```

## How to write docs with Hugo

### Add a new doc

- To create a new doc that contains all of the pre-configured Hugo front-matter and the docs task template:

    `hugo new <SECTIONNAME>/<FILENAME>.<FORMAT>`

  e.g.,

    hugo new install.md

  > The default template -- task -- should be used in most docs.
- To create other types of docs, you can add the `--kind` flag:
    `hugo new tutorials/deploy.md --kind tutorial`


The available kinds are:

- Task: Enable the customer to achieve a specific goal, based on use case scenarios.
- Concept: Help a customer learn about a specific feature or feature set.
- Reference: Describes an API, command line tool, config options, etc.; should be generated automatically from source code.
- Troubleshooting: Helps a customer solve a specific problem.
- Tutorial: Walk a customer through an example use case scenario; results in a functional PoC environment.

### Format internal links

Format links as [Hugo relrefs](https://gohugo.io/content-management/cross-references/).

> Note: Using file extensions when linking to internal docs with `relref` is optional.

- You can use relative paths or just the filename. We recommend using the filename
- Paths without a leading `/` are first resolved relative to the current page, then to the remainder of the site.
- Anchors are supported.

For example:

```md
To install NGINX Gateway Fabric, refer to the [installation instructions]({{< relref "/installation/install.md#section-1" >}}).
```

### Add images

You can use the `img` [shortcode](#use-hugo-shortcodes to insert images into your documentation.

1. Add the image to the static/img directory.
   DO NOT include a forward slash at the beginning of the file path. This will break the image when it's rendered.
   See the docs for the [Hugo relURL Function](https://gohugo.io/functions/relurl/#input-begins-with-a-slash) to learn more.

1. Add the img shortcode:

    {{< img src="img/<img-file.png>" >}}

> Note: The shortcode accepts all of the same parameters as the [Hugo figure shortcode](https://gohugo.io/content-management/shortcodes/#figure).

### Use Hugo shortcodes

You can use Hugo [shortcodes](https://gohugo.io/content-management/shortcodes) to do things like format callouts, add images, and reuse content across different docs.

For example, to use the note callout:

```md
{{< note >}}Provide the text of the note here. {{< /note >}}
```

The callout shortcodes also support multi-line blocks:

```md
{{< caution >}}
You should probably never do this specific thing in a production environment. If you do, and things break, don't say we didn't warn you.
{{< /caution >}}
```

Supported callouts:

- caution
- important
- note
- see-also
- tip
- warning

A few more useful shortcodes:

- collapse: makes a section collapsible
- table: adds scrollbars to wide tables when viewed in small browser windows or mobile browsers
- fa: inserts a Font Awesome icon
- include: include the content of a file in another file (requires the included file to be in the /includes directory)
- link: makes it possible to link to a static file and prepend the path with the Hugo baseUrl
- openapi: loads an OpenAPI spec and renders as HTML using ReDoc
- raw-html: makes it possible to include a block of raw HTML
- readfile: includes the content of another file in the current file; useful for adding code examples

## How to build docs locally

To view the docs in a browser, run the Hugo server. This will reload the docs automatically so you can view updates as you work.

> Note: The docs use build environments to control the baseURL that will be used for things like internal references and resource (CSS and JS) loading.
> You can view the config for each environment in the [config](./config) directory of this repo.
When running the Hugo server, you can specify the environment and baseURL if desired, but it's not necessary.

For example:

```shell
hugo server
```

```shell
hugo server -e development -b "http://127.0.0.1/nginx-gateway-fabric/"
```
