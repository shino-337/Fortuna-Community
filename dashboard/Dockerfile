FROM node:20-alpine AS build
WORKDIR /app

COPY dashboard/package*.json /app/
RUN npm ci

COPY dashboard/ /app/
# Tách bước để log build chỉ rõ lỗi (vite vs ghi version).
# date -Iseconds là GNU; trên Alpine/busybox dùng định dạng POSIX + UTC.
RUN npm run build
# Ghi version bằng Node (tránh sh/BuildKit parse sai chuỗi date có ký tự %).
RUN node -e "require('fs').writeFileSync('/app/dist/version.txt', 'build=' + new Date().toISOString() + ' utc\\n')"

FROM nginx:1.25-alpine
COPY dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
