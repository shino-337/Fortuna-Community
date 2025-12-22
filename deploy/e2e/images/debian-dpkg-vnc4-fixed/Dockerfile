FROM debian:10-slim

RUN mkdir -p /var/lib/dpkg
COPY status /var/lib/dpkg/status

CMD ["sh", "-c", "echo 'KSAM E2E dpkg fixed version image (debian base)' && sleep 3600"]


