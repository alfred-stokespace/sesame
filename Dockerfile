# syntax=docker/dockerfile:1

FROM golang:1.16-alpine as builder

RUN apk --no-cache add git

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

ADD cmd/ cmd/

RUN go build -o /sesame cmd/sesame/main.go

FROM golang:1.16-alpine as ssm-builder

RUN set -ex && apk update && apk add --no-cache make git gcc libc-dev curl bash zip && \
    mkdir -p /go/src/github.com && \
    cd /go/src/github.com/ && \
    git clone --depth 1 -b 1.2.398.0 https://github.com/aws/session-manager-plugin.git && \
    cd /go/src/github.com/session-manager-plugin && \
    make release

FROM alpine:3.17.2
RUN apk add --no-cache \
        ca-certificates \
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install --no-cache-dir \
        awscli \
    && rm -rf /var/cache/apk/*

RUN aws --version
# Copy our static executable.
COPY --from=builder /sesame /sesame
COPY github-based-automation-helper.sh /github-based-automation-helper.sh
COPY --from=ssm-builder /go/src/github.com/session-manager-plugin/bin/linux_amd64/ssmcli /usr/local/bin/

CMD [ "/sesame" ]