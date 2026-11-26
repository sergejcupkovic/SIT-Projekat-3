# Build context: repository root (docker-compose sets context: .), because
# go.mod replaces stashr/proto and stashr/shared with ../proto and ../shared.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY proto/ ./proto/
COPY shared/ ./shared/
COPY storage-service/go.mod storage-service/go.sum ./storage-service/
WORKDIR /src/storage-service
RUN go mod download
COPY storage-service/ ./
RUN CGO_ENABLED=0 go build -o /out/storage-service .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/storage-service /app/storage-service
EXPOSE 50052
CMD ["/app/storage-service"]
