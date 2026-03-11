FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /src
COPY . .

ARG VERSION=dev
ARG COMMIT=

ENV CGO_ENABLED=1

RUN go build -mod=vendor \
    -ldflags "-s -w -X repofalcon/internal/appinfo.Version=${VERSION} -X repofalcon/internal/appinfo.Commit=${COMMIT}" \
    -o /falcon ./cmd/falcon

FROM alpine:3.21

RUN apk add --no-cache ca-certificates git

COPY --from=builder /falcon /usr/local/bin/falcon

ENTRYPOINT ["falcon"]
