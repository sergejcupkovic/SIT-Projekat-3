FROM golang:1.27-alpine AS builder
WORKDIR /src

COPY proto/ ./proto/
COPY shared/ ./shared/
COPY notifications-service/go.mod notifications-service/go.sum* ./notifications-service/
WORKDIR /src/notifications-service
RUN go mod download

COPY notifications-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /notifications-service .

FROM alpine:3.21
COPY --from=builder /notifications-service /app/notifications-service
EXPOSE 50053
ENTRYPOINT ["/app/notifications-service"]