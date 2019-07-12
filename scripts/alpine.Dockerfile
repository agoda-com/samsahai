ARG USERNAME="samsahai"

FROM alpine:3.9

ARG USERNAME="samsahai"
ARG HTTP_PROXY=""
ARG HTTPS_PROXY=""
ARG NO_PROXY=""

COPY samsahai /usr/local/bin/
COPY staging /usr/local/bin/
COPY bin/kubectl-linux /usr/local/bin/kubectl

RUN set -ex; \
    \
    if [ ! -z "$HTTP_PROXY" ]; then \
        export http_proxy="$HTTP_PROXY"; \
        export https_proxy="$HTTP_PROXY"; \
        export no_proxy="$NO_PROXY"; \
    fi; \
    \
    apk add --no-cache ca-certificates tzdata; \
    \
    addgroup -g 1000 $USERNAME; \
    adduser -u 1000 -G $USERNAME -s /bin/sh -D $USERNAME; \
    \
    rm -rf /var/cache/apk/*; \
    \
    samsahai version;
