FROM golang:1.22 AS builder

ENV CGO_ENABLED=0

WORKDIR /go/src/app

ADD . .

RUN go build -o /bunker ./cmd/bunker

FROM scratch

WORKDIR /data

COPY --from=builder /bunker /bunker

CMD ["/bunker"]
