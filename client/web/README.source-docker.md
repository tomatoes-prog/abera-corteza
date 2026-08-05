# Source Docker Build

SPDX-License-Identifier: Apache-2.0

This Dockerfile builds the Corteza web applications from the local repository
source instead of downloading a prebuilt `corteza-webapp-*.tar.gz` release
archive.

Use it from the repository root:

```sh
docker build \
  -f client/web/Dockerfile.source \
  --build-arg WEBAPP_VERSION=dev-source \
  -t abera-corteza-webapp-source:dev \
  .
```

The build runs in two stages:

1. A Node/Yarn builder installs locked dependencies, builds `lib/js`, builds
   `lib/vue`, packs both local libraries as tarballs, installs those tarballs
   into each webapp, and runs production builds for `one`, `admin`, `compose`,
   `workflow`, `reporter`, `discovery`, and `privacy`.
2. A final Nginx image receives the generated `dist` output and reuses the
   existing `client/web/nginx.conf` and `client/web/entrypoint.sh`.

The local tarball step is intentional. It avoids the Docker-only
`@cortezaproject/corteza-js` resolution failure seen when `lib/vue` was built
with linked workspace packages. The source Dockerfile keeps the package lockfiles
unchanged and does not write generated frontend assets into the repository.

Useful local smoke test:

```sh
docker run -d --name corteza-webapp-source-test \
  -p 127.0.0.1:18084:80 \
  abera-corteza-webapp-source:dev

curl -I http://127.0.0.1:18084/config.js
curl -I http://127.0.0.1:18084/
curl -I http://127.0.0.1:18084/admin/
curl -I http://127.0.0.1:18084/compose/
curl -I http://127.0.0.1:18084/workflow/
curl -I http://127.0.0.1:18084/reporter/
curl -I http://127.0.0.1:18084/discovery/
curl -I http://127.0.0.1:18084/privacy/
```

Known non-blocking build warnings observed during validation:

- Browserlist/caniuse-lite data is stale.
- Dart Sass reports legacy API and `@import` deprecations.
- Vue CLI reports existing `no-console` ESLint findings as warnings in some
  webapps, but still emits production `dist` output.
- Several production bundles exceed Vue CLI's default size warning threshold.
