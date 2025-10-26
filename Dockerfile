FROM golang:1.25.3@sha256:6bac879c5b77e0fc9c556a5ed8920e89dab1709bd510a854903509c828f67f96 AS builder
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
