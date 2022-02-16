FROM golang:1.17 as build

RUN apt-get update && apt-get upgrade -y && apt-get install git -y

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /app ./...


FROM alpine:latest
EXPOSE 8080
COPY --from=build /app /usr/local/bin/app
CMD ["app"]