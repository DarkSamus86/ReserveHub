# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api-gateway ./cmd/api-gateway

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S reservehub \
    && adduser -S -G reservehub reservehub

COPY --from=build /out/api-gateway /usr/local/bin/api-gateway

USER reservehub
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api-gateway"]
