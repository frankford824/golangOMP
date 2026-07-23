ARG RUNTIME_IMAGE=alpine:3.20
FROM ${RUNTIME_IMAGE}

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 65532 ab-runtime \
    && adduser -S -D -H -u 65532 -G ab-runtime ab-runtime

WORKDIR /app
COPY --chmod=0555 ecommerce-api /app/ecommerce-api
COPY --chown=65532:65532 config/ /app/config/

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/ecommerce-api"]
