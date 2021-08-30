FROM alpine:3.14.1
ENV AUTHOR="jixingxing" PWD="."
ENV DEBIAN_FRONTEND=noninteractive
RUN set -x \
    && apk add --no-cache \
    haproxy

ENTRYPOINT ["/usr/sbin/haproxy", "-d", "-f", "/etc/haproxy/haproxy.cfg"]
