FROM golang:1.17-alpine as build

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /app ./main.go


FROM alpine:latest
EXPOSE 8080
COPY --from=build /app /usr/local/bin/app
CMD ["app"]