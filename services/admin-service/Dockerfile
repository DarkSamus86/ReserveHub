FROM golang:1.27-alpine AS build
WORKDIR /src
COPY services/tenant-service/go.mod services/tenant-service/go.sum ./services/tenant-service/
COPY services/admin-service/go.mod services/admin-service/go.sum ./services/admin-service/
WORKDIR /src/services/admin-service
RUN go mod download
WORKDIR /src
COPY services/tenant-service/gen ./services/tenant-service/gen
COPY services/admin-service ./services/admin-service
WORKDIR /src/services/admin-service
RUN CGO_ENABLED=0 go build -trimpath -o /admin-service ./cmd/admin-service

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /admin-service /admin-service
ENTRYPOINT ["/admin-service"]
