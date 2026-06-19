FROM 10.15.22.3:5005/distroless:debian13

COPY kgbrain /app/kgbrain
COPY docs /app/docs

WORKDIR /app
ENTRYPOINT ["/app/kgbrain", "--config", "/app/configs/config.toml"]
