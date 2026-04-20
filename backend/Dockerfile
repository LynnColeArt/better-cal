FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/better-cal-api ./cmd/api \
  && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/better-cal-worker ./cmd/worker

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app

COPY --from=build /out/better-cal-api /usr/local/bin/better-cal-api
COPY --from=build /out/better-cal-worker /usr/local/bin/better-cal-worker

USER app
EXPOSE 8080

CMD ["better-cal-api"]
