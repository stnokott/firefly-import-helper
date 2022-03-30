FROM alpine:latest
EXPOSE 8080
COPY app /usr/bin/app
CMD ["/usr/bin/app"]