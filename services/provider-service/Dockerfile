FROM golang:1.27-alpine AS build
WORKDIR /src
COPY services/tenant-service/go.mod services/tenant-service/go.sum ./services/tenant-service/
COPY services/provider-service/go.mod services/provider-service/go.sum ./services/provider-service/
WORKDIR /src/services/provider-service
RUN go mod download
WORKDIR /src
COPY services/tenant-service/gen ./services/tenant-service/gen
COPY services/provider-service ./services/provider-service
WORKDIR /src/services/provider-service
RUN CGO_ENABLED=0 go build -trimpath -o /provider-service ./cmd/provider-service

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /provider-service /provider-service
ENTRYPOINT ["/provider-service"]
