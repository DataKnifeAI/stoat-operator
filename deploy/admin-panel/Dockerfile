# Build DataKnifeAI/stoat-admin for Harbor.
# Pin a commit/tag for reproducible operator image builds.
ARG ADMIN_REF=main

FROM node:20-bookworm AS deps
ARG ADMIN_REF
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/* \
    && git clone --depth 1 --branch "${ADMIN_REF}" \
         https://github.com/DataKnifeAI/stoat-admin.git /app
RUN npm ci

FROM node:20-bookworm AS builder
WORKDIR /app
COPY --from=deps /app /app
ENV NEXT_TELEMETRY_DISABLED=1 \
    NODE_ENV=production \
    AUTHENTIK_ID=build \
    AUTHENTIK_SECRET=build \
    AUTHENTIK_ISSUER=https://example.invalid/application/o/stoat-admin \
    NEXTAUTH_SECRET=build-secret-not-for-production \
    NEXTAUTH_URL=http://localhost:3000 \
    MONGODB=mongodb://127.0.0.1:27017
RUN npm run build

FROM node:20-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOST=0.0.0.0 \
    PORT=3000
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r stoat && useradd -r -g stoat stoat
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
USER stoat
EXPOSE 3000
CMD ["node", "server.js"]
