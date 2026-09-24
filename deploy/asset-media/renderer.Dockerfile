FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates imagemagick poppler-utils tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY policy.xml /etc/ImageMagick-6/policy.xml
ENV ASSET_PREVIEW_RENDERER_BIN=/usr/bin/convert TZ=Asia/Shanghai
# Binaries are built by the reviewed release pipeline, mounted read-only.
ENTRYPOINT ["/app/bin/asset_media_nas_worker"]
