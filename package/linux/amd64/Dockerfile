FROM rancher/agent-base:v0.3.0
RUN mkdir /etc/rancher-dns/ && \
    touch /etc/rancher-dns/answers.json && \
    curl -sLf https://github.com/leodotcloud/loglevel/releases/download/v0.1/loglevel-amd64-v0.1.tar.gz | tar xvzf - -C /usr/bin
COPY rancher-dns /usr/bin/
COPY rancher-entrypoint.sh /
CMD ["rancher-dns"]
