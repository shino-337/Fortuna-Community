# syntax=docker/dockerfile:1
FROM node:20-alpine AS build
WORKDIR /app

COPY dashboard/package*.json /app/
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY dashboard/ /app/
# Split steps so build logs clearly separate Vite errors from version stamping.
# Use ISO UTC from Node in the next step (portable across Alpine/busybox).
RUN --mount=type=cache,target=/root/.npm \
    npm run build
# Write build stamp via Node (avoids shell/date quoting issues under BuildKit).
RUN node -e "require('fs').writeFileSync('/app/dist/version.txt', 'build=' + new Date().toISOString() + ' utc\\n')"

FROM nginx:1.25-alpine
COPY dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
