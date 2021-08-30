FROM ubuntu:latest
ENV AUTHOR="jixingxing" PWD="."
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt install keepalived -y
CMD service keepalived start  && tail -f /dev/null
