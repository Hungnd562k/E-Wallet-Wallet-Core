FROM golang:1.26-alpine as builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build api
RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/api ./api

ENV APP_ENV=local
ENV HTTP_ADDR=:8080
ENV DATABASE_URL=postgres://wallet:wallet@e-wallet-core-db:5432/wallet?sslmode=disable
ENV LOG_LEVEL=debug
ENV WORKER_ID=worker-1
ENV OUTBOX_BATCH_SIZE=20
ENV OUTBOX_POLL_INTERVAL=1s
ENV OUTBOX_MAX_ATTEMPTS=10

EXPOSE 8080

ENTRYPOINT [ "./api" ]