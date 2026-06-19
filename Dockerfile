FROM gcr.io/distroless/static-debian13:latest

COPY kgbrain /app/kgbrain
COPY docs /app/docs

WORKDIR /app
ENTRYPOINT ["/app/kgbrain", "--config", "/app/configs/config.toml"]
