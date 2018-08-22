# builder image
FROM golang:alpine as builder

RUN set -ex && apk --update --no-cache add \
    bash \
    make \
    git \
    cmake 

WORKDIR /go/src/github.com/DataReply/alertmanager-sns-forwarder
COPY . .
RUN make all

# final image
FROM golang:alpine
MAINTAINER o.grodzki@reply.de

RUN apk add -U python && rm -rf /var/cache/apk/*

COPY --from=builder /go/src/github.com/DataReply/alertmanager-sns-forwarder/bin/linux/alertmanager-sns-forwarder /bin/alertmanager-sns-forwarder

ENTRYPOINT ["/bin/alertmanager-sns-forwarder"]
