FROM ubuntu:16.04
COPY rancher-dns /usr/bin/
RUN mkdir /etc/rancher-dns
RUN touch /etc/rancher-dns/answers.json

CMD ["rancher-dns"]