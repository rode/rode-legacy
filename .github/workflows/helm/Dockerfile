FROM alpine:3.7

ENV HELM_LATEST_VERSION v3.1.0

RUN apk add -U ca-certificates git curl && \
    apk add -U -t deps curl && \
    curl -o helm.tar.gz https://get.helm.sh/helm-${HELM_LATEST_VERSION}-linux-amd64.tar.gz && \
    tar -xvf helm.tar.gz && \
    mv linux-amd64/helm /usr/local/bin && \
    chmod +x /usr/local/bin/helm && \
    rm -rf linux-amd64 && \
    rm helm.tar.gz && \
    apk del --purge deps && \
    rm /var/cache/apk/*

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
