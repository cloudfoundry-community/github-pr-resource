FROM golang AS builder
ADD . /go/src/github.com/telia-oss/github-pr-resource
WORKDIR /go/src/github.com/telia-oss/github-pr-resource
RUN curl -sL https://taskfile.dev/install.sh | TAG=v3.33.1 sh
RUN ./bin/task build

FROM alpine AS resource
COPY --from=builder /go/src/github.com/telia-oss/github-pr-resource/build /opt/resource
RUN apk add --update --no-cache \
    git \
    git-lfs \
    openssh \
    && chmod +x /opt/resource/*
COPY scripts/askpass.sh /usr/local/bin/askpass.sh
ADD scripts/install_git_crypt.sh install_git_crypt.sh
RUN ./install_git_crypt.sh && rm ./install_git_crypt.sh

FROM resource
LABEL MAINTAINER=telia-oss
