FROM golang:1.27-alpine AS build
WORKDIR /src
COPY proto/ ./proto/
COPY users-service/go.mod users-service/go.sum ./users-service/
WORKDIR /src/users-service
RUN go mod download
COPY users-service/ ./
RUN CGO_ENABLED=0 go build -o /out/users-service .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/users-service /app/users-service
EXPOSE 50051
CMD ["/app/users-service"]
