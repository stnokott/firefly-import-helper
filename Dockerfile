FROM golang:rc-alpine as build

RUN apk update && apk upgrade && apk add git

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /app ./...


FROM alpine:latest
EXPOSE 8080
COPY --from=build /app /usr/local/bin/app
CMD ["app"]