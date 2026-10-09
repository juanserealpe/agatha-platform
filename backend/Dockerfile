# Imagen del monolito Agatha. Por defecto compila cmd/platform, que levanta en un
# solo proceso los módulos elegidos en runtime con AGATHA_MODULES.
# APP permite compilar otro entrypoint de cmd/.
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG APP=platform

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o app \
    ./cmd/${APP}

FROM alpine:3.22

WORKDIR /app

# ca-certificates: TLS hacia DataOS y Supabase. tzdata: zona America/Bogota.
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S agatha \
    && adduser -S -G agatha agatha

COPY --from=builder /app/app .

USER agatha

EXPOSE 8080

CMD ["./app"]
