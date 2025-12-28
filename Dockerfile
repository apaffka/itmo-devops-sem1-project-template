FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -H -s /sbin/nologin app
WORKDIR /app
COPY --from=build /out/api /app/api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
