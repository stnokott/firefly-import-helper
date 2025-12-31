FROM alpine:3.23.2
RUN apk add --no-cache tzdata
EXPOSE 8888
ENV TZ=Europe/Berlin
COPY app /
CMD ["/app"]
