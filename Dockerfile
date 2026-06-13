FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /dependency-guardian ./cmd/action/

FROM node:22-alpine

COPY --from=builder /dependency-guardian /usr/local/bin/dependency-guardian

ENTRYPOINT ["dependency-guardian"]
