FROM alpine:3.17.0

RUN apk update && apk --no-cache add tzdata

ENV TZ=Europe/Berlin
EXPOSE 8822
COPY app /
ENTRYPOINT ["/app"]
