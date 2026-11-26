FROM golang:1.27-alpine AS build
WORKDIR /src
COPY proto/ ./proto/
COPY api-gateway/go.mod api-gateway/go.sum ./api-gateway/
WORKDIR /src/api-gateway
RUN go mod download
COPY api-gateway/ ./
RUN CGO_ENABLED=0 go build -o /out/api-gateway .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api-gateway /app/api-gateway
EXPOSE 8443
# TLS certificate + key are mounted at runtime (./certs -> /certs), never baked into the image.
CMD ["/app/api-gateway"]
