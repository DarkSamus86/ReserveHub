FROM golang:1.27-alpine AS build
WORKDIR /src
COPY services/tenant-service/go.mod services/tenant-service/go.sum ./services/tenant-service/
COPY services/provider-service/go.mod services/provider-service/go.sum ./services/provider-service/
COPY services/workspace-service/go.mod services/workspace-service/go.sum ./services/workspace-service/
WORKDIR /src/services/workspace-service
RUN go mod download
WORKDIR /src
COPY services/provider-service/gen ./services/provider-service/gen
COPY services/workspace-service ./services/workspace-service
WORKDIR /src/services/workspace-service
RUN CGO_ENABLED=0 go build -trimpath -o /workspace-service ./cmd/workspace-service

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /workspace-service /workspace-service
ENTRYPOINT ["/workspace-service"]
