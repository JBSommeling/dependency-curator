FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /dependency-curator ./cmd/action/

FROM node:22-alpine

RUN apk add --no-cache php83 php83-phar php83-mbstring php83-openssl php83-curl php83-json php83-iconv php83-zip php83-tokenizer php83-xmlwriter php83-xml php83-dom php83-simplexml composer go \
    && ln -sf /usr/bin/php83 /usr/bin/php

ENV GOPATH=/root/go
ENV PATH=$GOPATH/bin:$PATH

COPY --from=builder /dependency-curator /usr/local/bin/dependency-curator

ENTRYPOINT ["dependency-curator"]
