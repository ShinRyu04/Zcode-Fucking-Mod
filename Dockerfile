# syntax=docker/dockerfile:1
FROM golang:alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/zcode-bridge ./cmd/zcode-bridge

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S zcode \
 && adduser -S -G zcode -h /data zcode
COPY --from=build /out/zcode-bridge /usr/local/bin/zcode-bridge
WORKDIR /data
USER zcode
ENV HOST=0.0.0.0 \
    PORT=20131 \
    ZCODE_CONFIG=/data/config.yaml
EXPOSE 20131
VOLUME ["/data"]
ENTRYPOINT ["zcode-bridge"]
