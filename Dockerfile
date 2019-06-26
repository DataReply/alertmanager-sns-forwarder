# builder image
FROM golang:alpine as builder

RUN set -ex && apk --update --no-cache add \
    git \
    make

WORKDIR /go/src/github.com/DataReply/alertmanager-sns-forwarder
COPY . .
RUN make all

# final image
FROM scratch
MAINTAINER o.grodzki@reply.de

# Add sh and other tools for debugging the container
#COPY --from=builder /lib/ld-musl-x86_64.so.1 /lib/ld-musl-x86_64.so.1
#COPY --from=builder /bin/ /bin/

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/github.com/DataReply/alertmanager-sns-forwarder/bin/linux/alertmanager-sns-forwarder /bin/alertmanager-sns-forwarder

ENTRYPOINT ["/bin/alertmanager-sns-forwarder"]
