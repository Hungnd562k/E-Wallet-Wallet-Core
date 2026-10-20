FROM golang:1.26-alpine as builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build api
RUN CGO_ENABLE=0 GOOS=linux go build \
    -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/api ./api
COPY .env .env

EXPOSE 8080

ENTRYPOINT [ "./api" ]