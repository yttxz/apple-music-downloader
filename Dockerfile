ARG GOVERSION=1.25.5

FROM --platform=$BUILDPLATFORM golang:${GOVERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /bin/apple-music-dl .

FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates ffmpeg && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/apple-music-dl /usr/local/bin/apple-music-dl
WORKDIR /app
COPY config.yaml.example config.yaml
RUN sed -i 's|"AM-Lossless"|"/downloads/ALAC"|; s|"AM-Atmos"|"/downloads/Atmos"|; s|"AM-AAC"|"/downloads/AAC"|; s|"AM-MV"|"/downloads/MV"|' config.yaml
ENTRYPOINT ["/usr/local/bin/apple-music-dl"]
