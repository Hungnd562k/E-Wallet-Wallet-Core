FROM golang:1.26-alpine as builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build worker
RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/worker ./cmd/worker

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/worker ./worker

ENTRYPOINT [ "./worker" ]