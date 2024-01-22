FROM golang:1.21 AS builder
ENV CGO_ENABLED 0
ARG VERSION
WORKDIR /go/src/app
ADD . .
RUN go build -o /bunker

FROM debian:12

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y locales locales-all tzdata ca-certificates docker.io tini && \
    rm -rf /var/lib/apt/lists/*

EXPOSE 8080

COPY --from=builder /bunker /bunker

WORKDIR /data

ENTRYPOINT [ "tini", "--" ]

CMD ["/bunker"]
