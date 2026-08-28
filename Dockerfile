# Frontend shared-library stage. Application stages inherit the compiled Yarn
# links but keep independent source and dependency cache keys.
FROM node:22.22.0-bookworm@sha256:20a424ecd1d2064a44e12fe287bf3dae443aab31dc5e0c0cb6c74bef9c78911c AS webapp-libs

ARG BUILD_VERSION=dev
ENV BUILD_VERSION=${BUILD_VERSION}

WORKDIR /src

RUN corepack enable && corepack prepare yarn@1.22.22 --activate

# Shared packages are built and linked once. A shared-library change
# intentionally invalidates all web applications; an application-only change
# does not invalidate this stage or its siblings.
COPY lib ./lib

# Build and link each shared package once. The old lib/dev target built both
# packages twice before any application compilation started.
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd lib/js && yarn --frozen-lockfile --non-interactive && yarn build && yarn link && cd ../vue && yarn --frozen-lockfile --non-interactive && yarn cdeps && yarn build && yarn link

# Copy dependency manifests before application sources. This keeps dependency
# installation cached when only Vue, JavaScript or style sources change.
FROM webapp-libs AS webapp-admin
COPY client/web/admin/package.json client/web/admin/yarn.lock ./client/web/admin/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/admin && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/admin ./client/web/admin
RUN cd client/web/admin && yarn cdeps && yarn build

FROM webapp-libs AS webapp-compose
COPY client/web/compose/package.json client/web/compose/yarn.lock ./client/web/compose/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/compose && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/compose ./client/web/compose
RUN cd client/web/compose && yarn cdeps && yarn build

FROM webapp-libs AS webapp-discovery
COPY client/web/discovery/package.json client/web/discovery/yarn.lock ./client/web/discovery/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/discovery && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/discovery ./client/web/discovery
RUN cd client/web/discovery && yarn cdeps && yarn build

FROM webapp-libs AS webapp-one
COPY client/web/one/package.json client/web/one/yarn.lock ./client/web/one/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/one && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/one ./client/web/one
RUN cd client/web/one && yarn cdeps && yarn build

FROM webapp-libs AS webapp-privacy
COPY client/web/privacy/package.json client/web/privacy/yarn.lock ./client/web/privacy/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/privacy && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/privacy ./client/web/privacy
RUN cd client/web/privacy && yarn cdeps && yarn build

FROM webapp-libs AS webapp-reporter
COPY client/web/reporter/package.json client/web/reporter/yarn.lock ./client/web/reporter/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/reporter && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/reporter ./client/web/reporter
RUN cd client/web/reporter && yarn cdeps && yarn build

FROM webapp-libs AS webapp-workflow
COPY client/web/workflow/package.json client/web/workflow/yarn.lock ./client/web/workflow/
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn cd client/web/workflow && yarn --frozen-lockfile --non-interactive
COPY client/abera-assistant ./client/abera-assistant
COPY client/web/workflow ./client/web/workflow
RUN cd client/web/workflow && yarn cdeps && yarn build

# Assemble the exact runtime webapp tree without serially rebuilding clients.
FROM scratch AS webapp-build
COPY --from=webapp-one /src/client/web/one/dist/ /out/webapp/
COPY --from=webapp-admin /src/client/web/admin/dist/ /out/webapp/admin/
COPY --from=webapp-compose /src/client/web/compose/dist/ /out/webapp/compose/
COPY --from=webapp-discovery /src/client/web/discovery/dist/ /out/webapp/discovery/
COPY --from=webapp-privacy /src/client/web/privacy/dist/ /out/webapp/privacy/
COPY --from=webapp-reporter /src/client/web/reporter/dist/ /out/webapp/reporter/
COPY --from=webapp-workflow /src/client/web/workflow/dist/ /out/webapp/workflow/

# Server build stage. Only the upstream English locale is embedded. Additional
# browser locales are copied into the runtime image and loaded through
# LOCALE_PATH, so Spanish presentation edits do not rebuild the Go backend.
FROM golang:1.25.12-bookworm@sha256:6359592445455f2dbe2412bed411336035bc019a50017720d77454ffdd6d0f82 AS server-build

ARG BUILD_VERSION=dev
ARG SASS_VERSION=1.85.1
ARG SASS_SHA256=019a728e78c713caf32a6abc4501059de5ba26648d2e7f2f12ee7984e5a82389

WORKDIR /src

# This tool is independent from repository sources, so keep it ahead of COPY
# to avoid downloading and unpacking it after ordinary source changes.
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && curl --fail --silent --show-error --location \
       --retry 5 --retry-all-errors --retry-delay 2 \
       "https://github.com/sass/dart-sass/releases/download/${SASS_VERSION}/dart-sass-${SASS_VERSION}-linux-x64.tar.gz" \
       --output /tmp/dart-sass.tar.gz \
    && echo "${SASS_SHA256}  /tmp/dart-sass.tar.gz" | sha256sum --check --strict \
    && mkdir -p /out \
    && tar -xzf /tmp/dart-sass.tar.gz -C /out

# Keep template, demo and documentation edits out of the expensive Go build
# cache key. The server module only consumes its own tree plus embedded locale
# sources; templates are copied directly into the final runtime image below.
COPY server ./server
COPY locale/en ./locale/en

RUN rm -rf server/pkg/locale/src/en server/pkg/locale/src/es \
    && mkdir -p server/pkg/locale/src \
    && cp -a locale/en server/pkg/locale/src/en

RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out/bin \
    && cd server \
    && CGO_ENABLED=1 go build -mod=vendor -trimpath \
       -ldflags "-X github.com/cortezaproject/corteza/server/pkg/version.Version=${BUILD_VERSION}" \
       -o /out/bin/corteza-server ./cmd/corteza/main.go

# Optional validation target:
# docker build --target server-test .
FROM server-build AS server-test

RUN --mount=type=cache,target=/root/.cache/go-build \
    cd server \
    && CGO_ENABLED=1 go test -mod=vendor ./pkg/provision ./auth/oauth2

# Runtime stage
FROM ubuntu:22.04@sha256:3b06811b2afd352be909dd088a004166d665dc76d38b13eada33522a9d915c6f

ARG BUILD_VERSION=dev

LABEL org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.documentation="TRANSLATIONS.md" \
      org.opencontainers.image.source="https://github.com/tomatoes-prog/abera-corteza" \
      org.opencontainers.image.version="${BUILD_VERSION}"

RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl sed \
    && rm -rf /var/lib/apt/lists/*

# Persist the SQLite primary store in the declared volume. Corteza otherwise
# defaults to an in-memory SQLite database that can lose its schema after idle
# connections close, causing OAuth and background services to fail.
ENV DB_DSN="sqlite3://file:/data/corteza.db?cache=shared&mode=rwc"
ENV UPGRADE_ALWAYS="true"
ENV STORAGE_PATH="/data"
ENV CORREDOR_ADDR="corredor:80"
ENV HTTP_ADDR="0.0.0.0:80"
# Keep absolute OAuth URLs aligned with the documented local Docker mapping.
# Override DOMAIN and DOMAIN_WEBAPP when publishing the container elsewhere.
ENV DOMAIN="localhost:8080"
ENV DOMAIN_WEBAPP="localhost:8080"
ENV HTTP_WEBAPP_ENABLED="true"
ENV HTTP_WEBAPP_BASE_DIR="/corteza/webapp"
ENV LOCALE_LANGUAGES="es,en"
ENV LOCALE_PATH="/corteza/locale"
ENV CORTEZA_DEFAULT_LOCALE="es"
ENV PATH="/opt/dart-sass:/corteza/bin:${PATH}"

WORKDIR /corteza

VOLUME /data

COPY --from=server-build /out/bin/corteza-server ./bin/corteza-server
COPY --from=server-build /out/dart-sass /opt/dart-sass
COPY --from=server-build /src/server/provision ./provision
COPY locale ./locale
COPY --from=webapp-build /out/webapp ./webapp
COPY templates ./templates
COPY demos ./demos
COPY docker-template-select.sh /template-select.sh
COPY docker-entrypoint.sh /entrypoint.sh
COPY LICENSE NOTICE README.md CHANGELOG.md TRANSLATIONS.md TEMPLATES.md BOOTSTRAP_OAUTH.md AI_ASSISTANT.md ./

RUN chmod +x /entrypoint.sh /template-select.sh

HEALTHCHECK --interval=30s --start-period=1m --timeout=30s --retries=3 \
    CMD curl --silent --fail --fail-early http://127.0.0.1:80/healthcheck || exit 1

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]

CMD ["serve-api"]
