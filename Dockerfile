FROM golang:1.16-bullseye AS builder

RUN apt-get update

# sibench build dependencies
RUN apt-get install -y librbd-dev librados-dev make git

# copy source code
COPY . /src/sibench

# build and install Sibench
RUN cd /src/sibench && make
RUN cp /src/sibench/bin/sibench /usr/local/bin

FROM debian:bullseye-slim
ARG DEBIAN_FRONTEND=noninteractive

# sibench/benchmaster run dependencies
RUN apt-get update && apt-get install -y \
  librados2 \
  librbd1 \
  python3-pip \
  && rm -rf /var/lib/apt/lists/*

# install benchmaster
RUN python3 -m pip install https://github.com/SoftIron/benchmaster/archive/refs/heads/master.tar.gz

# install previously built sibench
COPY --from=builder /src/sibench/bin/sibench /usr/local/bin
