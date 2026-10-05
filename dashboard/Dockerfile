# syntax=docker/dockerfile:1
FROM node:24-alpine AS build
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

FROM nginx:1.30.5-alpine3.24
RUN apk upgrade --no-cache
# Rendered to /etc/nginx/conf.d/default.conf by the nginx entrypoint; only the
# FORTUNA_* variables and NGINX_LOCAL_RESOLVERS are substituted.
COPY dashboard/nginx/default.conf.template /etc/nginx/templates/default.conf.template
ENV FORTUNA_CORE_HOST=fortuna-core.fortuna.svc.cluster.local \
    NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 \
    NGINX_ENVSUBST_FILTER="^(FORTUNA_|NGINX_LOCAL_RESOLVERS)"
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
