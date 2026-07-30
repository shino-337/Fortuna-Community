# Alpine 3.12 with openssl for SBOM/CVE test. Repo may serve 1.1.1o; pin 1.1.1g only if using older apk index.
FROM alpine:3.12

RUN apk update && \
    apk add --no-cache openssl

CMD ["sleep", "3600"]
