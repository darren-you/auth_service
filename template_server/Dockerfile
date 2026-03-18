# syntax=docker/dockerfile:1.7
# check=skip=InvalidDefaultArgInFrom
ARG BASE_IMAGE_REGISTRY
ARG BUILDER_BASE_IMAGE
ARG RUNTIME_BASE_IMAGE
ARG PROJECT_NAME
ARG BUILD_ENV

FROM --platform=$BUILDPLATFORM ${BASE_IMAGE_REGISTRY}/${BUILDER_BASE_IMAGE} AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

COPY . .

ARG PROJECT_NAME
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/${PROJECT_NAME} ./template_server/cmd/server

FROM --platform=$TARGETPLATFORM ${BASE_IMAGE_REGISTRY}/${RUNTIME_BASE_IMAGE}

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

ARG BUILD_ENV
ARG PROJECT_NAME
ARG REMOTE_CONTAINER_PORT
ENV PROJECT_NAME=${PROJECT_NAME}

COPY --from=builder /out/ /out/
COPY --from=builder /src/template_server/config ./config
RUN case "$BUILD_ENV" in \
      prod) cp ./config/config.prod.yaml ./config/config.yaml ;; \
      test) cp ./config/config.dev.yaml ./config/config.yaml ;; \
      *) echo "unsupported BUILD_ENV: $BUILD_ENV" >&2; exit 1 ;; \
    esac
RUN cp "/out/${PROJECT_NAME}" /usr/local/bin/server && rm -rf /out

EXPOSE ${REMOTE_CONTAINER_PORT}

ENTRYPOINT ["server"]
