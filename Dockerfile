FROM golang:1.26.7-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o api ./cmd/main.go

FROM alpine:3.23.3

RUN apk add --no-cache ca-certificates && \
    addgroup -S api-user && \
    adduser -S api-user -G api-user

WORKDIR /app
COPY --from=builder /src/api ./api

USER api-user
EXPOSE 8080

CMD ["./api"]
