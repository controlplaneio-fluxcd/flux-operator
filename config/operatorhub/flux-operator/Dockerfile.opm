FROM alpine:3.20

ARG DOCKER_VERSION=26.1.3
ARG OPM_VERSION=4.15.16
# x86_64 or aarch64
ARG ARCH=x86_64

WORKDIR /opt
RUN wget https://download.docker.com/linux/static/stable/${ARCH}/docker-${DOCKER_VERSION}.tgz
RUN tar xf docker-${DOCKER_VERSION}.tgz

RUN wget https://mirror.openshift.com/pub/openshift-v4/${ARCH}/clients/ocp/${OPM_VERSION}/opm-linux-${OPM_VERSION}.tar.gz
RUN tar xf opm-linux-${OPM_VERSION}.tar.gz

FROM ubuntu:24.04

WORKDIR /opt

COPY --from=0 /opt/docker/docker /usr/bin/
COPY --from=0 /opt/opm /opt/

ENTRYPOINT ["/opt/opm"]
