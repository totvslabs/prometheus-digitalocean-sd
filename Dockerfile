FROM gcr.io/distroless/base
COPY prometheus-digitalocean-sd /prometheus-digitalocean-sd
ENTRYPOINT ["/prometheus-digitalocean-sd"]

