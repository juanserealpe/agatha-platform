FROM golang:1.25.7-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG APP

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o app \
    ./cmd/${APP}

FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/app .

CMD ["./app"]