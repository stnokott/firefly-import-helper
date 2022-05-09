FROM golang:1.18.1-alpine as build

RUN apk update && apk add git

WORKDIR /usr/src/firefly-import-helper

COPY . ./
RUN go mod download && go mod verify

RUN CGO_ENABLED=0 GOOS=linux go build -v -o app .

FROM alpine:latest
EXPOSE 8080
WORKDIR /root/
COPY --from=0 /usr/src/firefly-import-helper/app ./
CMD ["./app"]