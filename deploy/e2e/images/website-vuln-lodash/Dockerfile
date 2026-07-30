# Static website with vulnerable npm dependency (lodash 4.17.19 - CVE-2020-8203 etc.) for SBOM/CVE scan
FROM node:18-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install --omit=dev

COPY public/ ./public/

# serve static files on port 3000, long-running
EXPOSE 3000
CMD ["npx", "serve", "public", "-l", "3000"]
