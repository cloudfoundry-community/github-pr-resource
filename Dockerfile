FROM public.ecr.aws/docker/library/golang:1.22 as builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y -qq update \
    && apt-get -y -qq install "make"

ADD . /go/src/github.com/telia-oss/github-pr-resource
WORKDIR /go/src/github.com/telia-oss/github-pr-resource

RUN go version &&  make all

FROM public.ecr.aws/docker/library/alpine:3.21.3 AS resource
RUN apk add --update --no-cache \
    git \
    git-lfs \
    curl \
    openssh \
    git-crypt
COPY scripts/askpass.sh /usr/local/bin/askpass.sh
ENV GITHUB_APP_CRED_HELPER_VERSION="v0.3.2"
ENV BIN_PATH_TARGET=/usr/local/bin
RUN curl -L https://github.com/bdellegrazie/git-credential-github-app/releases/download/${GITHUB_APP_CRED_HELPER_VERSION}/git-credential-github-app_${GITHUB_APP_CRED_HELPER_VERSION}_Linux_x86_64.tar.gz | tar zxv -C ${BIN_PATH_TARGET}
COPY --from=builder /go/src/github.com/telia-oss/github-pr-resource/build /opt/resource
RUN chmod +x /opt/resource/*

FROM resource
LABEL MAINTAINER=cloudfoundry-community
