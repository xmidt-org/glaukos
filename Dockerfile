FROM docker.io/library/golang:1.15-alpine as builder

MAINTAINER Jack Murdock <jack_murdock@comcast.com>

WORKDIR /src

ARG VERSION
ARG GITCOMMIT
ARG BUILDTIME


RUN apk add --no-cache --no-progress \
    ca-certificates \
    make \
    git \
    openssh \
    gcc \
    libc-dev \
    upx

RUN go get github.com/geofffranks/spruce/cmd/spruce && chmod +x /go/bin/spruce
COPY . .
RUN make test release

FROM alpine:3.12.1

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/glaukos /src/glaukos.yaml /src/deploy/packaging/entrypoint.sh /go/bin/spruce /src/Dockerfile /src/NOTICE /src/LICENSE /src/CHANGELOG.md /
COPY --from=builder /src/deploy/packaging/glaukos.yaml /tmp/glaukos.yaml

RUN mkdir /etc/glaukos/ && touch /etc/glaukos/glaukos.yaml && chmod 666 /etc/glaukos/glaukos.yaml

USER nobody

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 4200
EXPOSE 4201
EXPOSE 4202
EXPOSE 4203

CMD ["/glaukos"]
