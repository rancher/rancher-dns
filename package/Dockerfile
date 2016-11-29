FROM rancher/agent-base:v0.2.0
RUN mkdir /etc/rancher-dns/ && \
    touch /etc/rancher-dns/answers.json
COPY rancher-dns /usr/bin/
CMD ["rancher-dns"]
