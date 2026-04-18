FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/better-cal-api ./cmd/api

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app

COPY --from=build /out/better-cal-api /usr/local/bin/better-cal-api

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=6 --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["better-cal-api"]
