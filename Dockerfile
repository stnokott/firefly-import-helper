FROM golang:1.20.2-alpine as build

RUN apk update && apk --no-cache add git tzdata

WORKDIR /usr/src/firefly-import-helper

COPY . ./
RUN go mod download && go mod verify

RUN CGO_ENABLED=0 GOOS=linux go build -v -o app .

FROM alpine:latest
EXPOSE 8822
WORKDIR /root/
COPY --from=build /usr/src/firefly-import-helper/app ./
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Europe/Berlin
CMD ["./app"]
