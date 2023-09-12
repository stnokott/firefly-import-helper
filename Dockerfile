FROM alpine:3.17.2
RUN apk add --no-cache tzdata
EXPOSE 8822
ENV TZ=Europe/Berlin
COPY app /
CMD ["/app"]
