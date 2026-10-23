# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-service ./cmd/auth-service

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && addgroup -S reservehub \
    && adduser -S -G reservehub reservehub
COPY --from=build /out/auth-service /usr/local/bin/auth-service
USER reservehub
EXPOSE 8082
ENTRYPOINT ["/usr/local/bin/auth-service"]
