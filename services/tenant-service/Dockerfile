# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tenant-service ./cmd/tenant-service

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S reservehub \
    && adduser -S -G reservehub reservehub

COPY --from=build /out/tenant-service /usr/local/bin/tenant-service

USER reservehub
EXPOSE 8081 9091

ENTRYPOINT ["/usr/local/bin/tenant-service"]
