FROM golang:1.15-alpine

RUN apk add --no-cache gcc libc-dev

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
