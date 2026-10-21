# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/nucleaus ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/nucleaus /app/nucleaus
COPY --chown=app:app docs /app/docs
COPY --chown=app:app internal/web/tts /app/internal/web/tts
USER app
EXPOSE 9088
ENTRYPOINT ["/app/nucleaus"]
