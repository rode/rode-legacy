FROM gcr.io/k8s-skaffold/skaffold:v1.3.1

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
