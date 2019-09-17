# https://hub.docker.com/_/alpine
FROM alpine:edge as build1

MAINTAINER Instrumentisto Team <developer@instrumentisto.com>


# Build and install Coturn
RUN apk update \
 && apk upgrade \
 && apk add --no-cache \
 # for golang network
   libc6-compat \
        ca-certificates \
        curl \
 && update-ca-certificates \
    \
 # Install Coturn dependencies
 && apk add --no-cache \
        libevent \
        libcrypto1.1 libssl1.1 \
    \
 # Install tools for building
 && apk add --no-cache --virtual .tool-deps \
        coreutils autoconf g++ libtool make \
    \
 # Install Coturn build dependencies
 && apk add --no-cache --virtual .build-deps \
        linux-headers \
        libevent-dev \
        openssl-dev \
    \
 # Download and prepare Coturn sources
 && curl -fL -o /tmp/coturn.tar.gz \
         https://github.com/coturn/coturn/archive/4.5.1.1.tar.gz \
 && tar -xzf /tmp/coturn.tar.gz -C /tmp/ \
 && cd /tmp/coturn-* \
    \
 # Build Coturn from sources
 && ./configure --prefix=/usr \
        --turndbdir=/var/lib/coturn \
        --disable-rpath \
        --sysconfdir=/etc/coturn \
        # No documentation included to keep image size smaller
        --mandir=/tmp/coturn/man \
        --docsdir=/tmp/coturn/docs \
        --examplesdir=/tmp/coturn/examples \
 && make bin/turnutils_uclient \
    \
 # Install and configure Coturn
 # && make install \
 # Preserve license file
 && mkdir -p /usr/share/licenses/coturn/ \
 && cp LICENSE /usr/share/licenses/coturn/ \ 
 && cp bin/turnutils_uclient /usr/local/bin \
    \
 # Cleanup unnecessary stuff
 && apk del .tool-deps .build-deps \
 && rm -rf /var/cache/apk/* \
           /tmp/*

COPY rootfs /

RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
             /usr/local/bin/detect-external-ip.sh \
 && ln -s /usr/local/bin/detect-external-ip.sh \
          /usr/local/bin/detect-external-ip


EXPOSE 9090


RUN apk add --no-cache git make musl-dev go
ENV GOROOT /usr/lib/go
ENV GOPATH /go
ENV PATH /go/bin:$PATH
RUN mkdir -p ${GOPATH}/src ${GOPATH}/bin

RUN go get -u github.com/limertc/turnmonitorx/... 
RUN cd /go/src/github.com/limertc/turnmonitorx \
&& CGO_ENABLED=1 GOOS=linux go build -o main \
&& mkdir /app \
&& cp main /app
RUN apk add --no-cache bash
#RUN  CGO_ENABLED=1 GOOS=linux go install -a github.com/limertc/turnmonitorx
#RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .
#COPY turnmonitorx /app/

VOLUME ["/var/lib/coturn"]

#ENTRYPOINT ["docker-entrypoint.sh"]

#CMD ["-n", "--log-file=stdout", "--external-ip=$(detect-external-ip)"]
ENTRYPOINT ["/app/main"]