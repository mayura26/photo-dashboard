# Coolify / Docker: self-contained Go server + web/ + household config.
# Version: APP_VERSION build-arg → SOURCE_COMMIT (Coolify) → git SHA → "dev".
# .dockerignore excludes .git, so git fallback only works when .git is present.
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG APP_VERSION=
ARG SOURCE_COMMIT=
RUN VERSION="${APP_VERSION}" \
	&& if [ -z "$VERSION" ] && [ -n "$SOURCE_COMMIT" ]; then VERSION="$SOURCE_COMMIT"; fi \
	&& if [ -z "$VERSION" ]; then VERSION=$(git rev-parse --short HEAD 2>/dev/null || true); fi \
	&& if [ -z "$VERSION" ]; then VERSION=dev; fi \
	&& VERSION=$(printf '%s' "$VERSION" | cut -c1-7) \
	&& printf '{"version":"%s"}\n' "$VERSION" > web/version.json \
	&& echo "$VERSION" > /out-version
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /src/web /app/web
COPY --from=build /out-version /app/VERSION
COPY internal/hq/household.json /app/internal/hq/household.json
COPY go.mod /app/go.mod

# Do not bake an empty APP_VERSION into the image ENV — that blocks Coolify's
# runtime SOURCE_COMMIT from being used. Resolve at container start instead.
# tzdata + TZ so time.Local / LoadLocation observe AEDT/AEST (not fixed UTC+10).
ENV PORT=8080
ENV TZ=Australia/Sydney
EXPOSE 8080
CMD ["/bin/sh", "-c", "V=\"${APP_VERSION:-${SOURCE_COMMIT:-$(cat /app/VERSION)}}\"; V=$(printf '%s' \"$V\" | cut -c1-7); export APP_VERSION=\"$V\"; exec /app/server"]
