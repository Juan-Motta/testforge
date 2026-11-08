# One image for the API, the worker and the CLI; the compose command picks the binary.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ ./cmd/api ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X testforge/internal/cli.Version=${VERSION}" -o /out/testforge ./cmd/testforge

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 testforge \
 && mkdir -p /data/artifacts && chown -R testforge /data
COPY --from=build /out/ /usr/local/bin/
USER testforge
ENV HTTP_ADDR=:8080 ARTIFACTS_DIR=/data/artifacts
EXPOSE 8080
CMD ["api", "serve"]
