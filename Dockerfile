FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /dependency-curator ./cmd/action/

FROM node:22-alpine

COPY --from=builder /dependency-curator /usr/local/bin/dependency-curator

ENTRYPOINT ["dependency-curator"]
