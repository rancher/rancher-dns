FROM rancher/agent-base:v0.3.0
RUN mkdir /etc/rancher-dns/ && \
    touch /etc/rancher-dns/answers.json
COPY rancher-dns /usr/bin/
COPY rancher-entrypoint.sh /
CMD ["rancher-dns"]
