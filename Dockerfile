# Coolify / Docker: self-contained Go server + web/ + seed.
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY web /app/web
COPY internal/demo/seed.json /app/internal/demo/seed.json
COPY go.mod /app/go.mod

ENV PORT=8080
EXPOSE 8080
CMD ["/app/server"]
