# Coolify / Docker: self-contained Go server + web/ + seed.
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG APP_VERSION=
RUN VERSION="${APP_VERSION}" \
	&& if [ -z "$VERSION" ]; then VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo dev); fi \
	&& printf '{"version":"%s"}\n' "$VERSION" > web/version.json \
	&& echo "$VERSION" > /out-version
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /src/web /app/web
COPY --from=build /out-version /app/VERSION
COPY internal/demo/seed.json /app/internal/demo/seed.json
COPY go.mod /app/go.mod

ARG APP_VERSION=
# Prefer build-arg when set; otherwise entrypoint exports the stamped file.
ENV APP_VERSION=${APP_VERSION}
ENV PORT=8080
EXPOSE 8080
CMD ["/bin/sh", "-c", "export APP_VERSION=\"${APP_VERSION:-$(cat /app/VERSION)}\"; exec /app/server"]
