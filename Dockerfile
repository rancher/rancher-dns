FROM golang:1.6
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
