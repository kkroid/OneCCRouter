# Base on copilot-api which has bun runtime
FROM ghcr.io/ericc-ch/copilot-api:latest

WORKDIR /app
COPY package.json ./
RUN bun install --no-cache

COPY src/ ./src/
COPY models.conf ./

EXPOSE 4142
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --spider -q http://127.0.0.1:4142/health || exit 1

ENTRYPOINT []
CMD ["bun", "run", "src/index.ts"]
