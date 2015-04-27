FROM golang:1.4.2
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
