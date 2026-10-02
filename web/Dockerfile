# syntax=docker/dockerfile:1
# HookReplay dashboard — Next.js 14.
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:20-alpine AS build
WORKDIR /app
# Next.js bakes the rewrite target into the build, so it must be a build arg
# (not a runtime env var). Defaults to the compose service name `api`.
ARG HOOKREPLAY_API_PROXY=http://api:8080
ENV HOOKREPLAY_API_PROXY=${HOOKREPLAY_API_PROXY}
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=deps /app/node_modules ./node_modules
COPY --from=build /app/.next ./.next
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/next.config.js ./next.config.js
EXPOSE 3000
CMD ["npm", "start"]
