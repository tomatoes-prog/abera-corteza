# Frontend build stage. The previous Dockerfile downloaded a 2022.9.0
# prebuilt webapp, which meant repository changes could not affect the image.
FROM node:20.18-bookworm AS webapp-build

ARG BUILD_VERSION=2024.9.9-hotfix.1
ENV BUILD_VERSION=${BUILD_VERSION}

WORKDIR /src

RUN corepack enable && corepack prepare yarn@1.22.22 --activate

COPY . .

# Build local libraries first and link them into every web application so the
# runtime locale configuration in lib/vue is included in the bundles.
RUN make -C lib dev
RUN make -C client dev
RUN make -C client build

RUN mkdir -p /out/webapp/admin /out/webapp/compose /out/webapp/discovery \
    /out/webapp/privacy /out/webapp/reporter /out/webapp/workflow
RUN cp -a client/web/one/dist/. /out/webapp/
RUN cp -a client/web/admin/dist/. /out/webapp/admin/
RUN cp -a client/web/compose/dist/. /out/webapp/compose/
RUN cp -a client/web/discovery/dist/. /out/webapp/discovery/
RUN cp -a client/web/privacy/dist/. /out/webapp/privacy/
RUN cp -a client/web/reporter/dist/. /out/webapp/reporter/
RUN cp -a client/web/workflow/dist/. /out/webapp/workflow/

# Server build stage. Locale files are copied into the embedded filesystem so
# the image remains self-contained and does not depend on a mounted locale dir.
FROM golang:1.25-bookworm AS server-build

ARG SASS_VERSION=1.69.5

WORKDIR /src

COPY . .

RUN rm -rf server/pkg/locale/src/en server/pkg/locale/src/es \
    && mkdir -p server/pkg/locale/src \
    && cp -a locale/en server/pkg/locale/src/en \
    && cp -a locale/es server/pkg/locale/src/es

RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out/bin \
    && cd server \
    && CGO_ENABLED=1 go build -mod=vendor -trimpath \
       -ldflags "-X github.com/cortezaproject/corteza/server/pkg/version.Version=2024.9.9-hotfix.1" \
       -o /out/bin/corteza-server ./cmd/corteza/main.go

RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && curl --fail --silent --show-error --location \
       "https://github.com/sass/dart-sass/releases/download/${SASS_VERSION}/dart-sass-${SASS_VERSION}-linux-x64.tar.gz" \
       --output /tmp/dart-sass.tar.gz \
    && mkdir -p /out \
    && tar -xzf /tmp/dart-sass.tar.gz -C /out

# Optional validation target:
# docker build --target server-test .
FROM server-build AS server-test

RUN --mount=type=cache,target=/root/.cache/go-build \
    cd server \
    && CGO_ENABLED=1 go test -mod=vendor ./pkg/provision ./auth/oauth2

# Runtime stage
FROM ubuntu:22.04

LABEL org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.documentation="TRANSLATIONS.md"

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
COPY --from=server-build /src/locale ./locale
COPY --from=webapp-build /out/webapp ./webapp
COPY templates ./templates
COPY demos ./demos
COPY docker-template-select.sh /template-select.sh
COPY docker-entrypoint.sh /entrypoint.sh
COPY LICENSE NOTICE README.md CHANGELOG.md TRANSLATIONS.md TEMPLATES.md BOOTSTRAP_OAUTH.md ./

RUN chmod +x /entrypoint.sh /template-select.sh

HEALTHCHECK --interval=30s --start-period=1m --timeout=30s --retries=3 \
    CMD curl --silent --fail --fail-early http://127.0.0.1:80/healthcheck || exit 1

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]

CMD ["serve-api"]
