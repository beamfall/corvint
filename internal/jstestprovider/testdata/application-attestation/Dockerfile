FROM scratch
COPY fixture-server /fixture-server
HEALTHCHECK --interval=250ms --timeout=1s --retries=20 CMD ["/fixture-server", "healthcheck"]
ENTRYPOINT ["/fixture-server"]
