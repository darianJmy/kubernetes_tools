FROM alpine:3.14.1
ENV AUTHOR="jixingxing" PWD="."
ENV DEBIAN_FRONTEND=noninteractive
RUN set -x \
    && apk add --no-cache \
    keepalived \
    socat \
    nftables

ENTRYPOINT ["/usr/sbin/keepalived", "-l", "-n"]
