FROM alpine:3.17.2
EXPOSE 8822
ENV TZ=Europe/Berlin
COPY app /
CMD ["/app"]
