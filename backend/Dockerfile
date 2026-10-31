FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/api /api

EXPOSE 8080

ENTRYPOINT ["/api"]
