FROM golangci/golangci-lint:v1.23.6

RUN apt-get install git

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
