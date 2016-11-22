FROM ubuntu:16.04
RUN apt-get update && \
    apt-get install -y iptables iproute2
RUN mkdir /etc/rancher-dns/ && \
    touch /etc/rancher-dns/answers.json
COPY rancher-dns /usr/bin/
CMD ["rancher-dns"]
