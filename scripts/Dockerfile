ARG WORKDIR="/go/src/github.com/agoda-com/samsahai"
ARG USERNAME="samsahai"

FROM golang:1.17-alpine as builder

ARG WORKDIR
ARG GO_LDFLAGS

ENV CGO_ENABLED=0

RUN set -ex; \
    apk add --no-cache bash make git; \
    go get golang.org/x/tools/cmd/goimports; \
    go get github.com/ahmetb/govvv; \
    command -v govvv;

WORKDIR $WORKDIR

COPY ["go.mod", "go.sum", "./"]

ENV GO111MODULE=on

# download dependencies
RUN go mod tidy

ADD . .

# build
RUN make build go_ldflags="$GO_LDFLAGS" \
  && make build-staging-ctrl go_ldflags="$GO_LDFLAGS"

# final stage
FROM alpine:3.9

ARG WORKDIR
ARG USERNAME

WORKDIR /home/agoda
COPY --from=builder $WORKDIR/out/samsahai /usr/local/bin/
COPY --from=builder $WORKDIR/out/staging-ctrl /usr/local/bin/

RUN set -ex; \
    \
    apk add --no-cache ca-certificates; \
    \
    addgroup -g 1000 $USERNAME; \
    adduser -u 1000 -G $USERNAME -s /bin/sh -D $USERNAME; \
    \
    rm -rf /var/cache/apk/*; \
    \
    samsahai version;