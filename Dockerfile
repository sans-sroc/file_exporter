FROM golang:1.23.3@sha256:e5ca1999e21764b1fd40cf6564ebfb7022e7a55b8c72886a9bcb697a5feac8d6 AS builder
WORKDIR /src/file_exporter
ENV GO111MODULE=on
COPY . /src/file_exporter
ARG branch=master
ENV BRANCH=${branch}
RUN make release && cp release/file_exporter /go/bin/file_exporter

FROM cgr.dev/chainguard/wolfi-base:latest@sha256:52f88fede0eba350de7be98a4a803be5072e5ddcd8b5c7226d3ebbcd126fb388 as base
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/bin/file_exporter"]
RUN apk update && apk add tini
RUN adduser -D -u 999 file_exporter
USER file_exporter

FROM base as goreleaser
COPY file_exporter /usr/bin/file_exporter

FROM base
COPY --from=builder /go/bin/file_exporter /usr/bin/file_exporter
