# TODO: extract builder and base image into its own repo for reuse and to speed up builds
FROM docker.io/library/ubuntu:20.04 AS builder
SHELL ["/bin/bash", "-euEo", "pipefail", "-c"]
ENV GOPATH=/go \
    GOROOT=/usr/local/go \
# Enable madvdontneed=1, for golang < 1.16 https://github.com/golang/go/issues/42330
    GODEBUG=madvdontneed=1
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
RUN apt-get update; \
    apt-get install -y build-essential git curl; \
    apt-get clean; \
    curl --fail -L https://storage.googleapis.com/golang/go1.15.7.linux-amd64.tar.gz | tar -C /usr/local -xzf -
WORKDIR /go/src/github.com/scylladb/scylla-operator-autoscaler
COPY . .
RUN make build --warn-undefined-variables

FROM docker.io/library/ubuntu:20.04
ARG EXEC_ARG
ENV EXEC=$EXEC_ARG
SHELL ["/bin/bash", "-euEo", "pipefail", "-c"]
COPY --from=builder /go/src/github.com/scylladb/scylla-operator-autoscaler/$EXEC_ARG /usr/bin/
ENTRYPOINT ./usr/bin/$EXEC
