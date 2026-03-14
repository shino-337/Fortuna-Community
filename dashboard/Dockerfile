FROM node:20-alpine AS build
WORKDIR /app

COPY dashboard/package*.json /app/
RUN npm install

COPY dashboard/ /app/
RUN npm run build && echo "build=$(date -Iseconds) utc" > /app/dist/version.txt

FROM nginx:1.25-alpine
COPY dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
