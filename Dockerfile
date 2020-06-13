FROM golang:1.14 as build

WORKDIR /go/src/app
ADD . .

RUN go install cmd/main.go && mv /go/bin/main /go/lunchbox

FROM scratch
WORKDIR /app
COPY --from=build /go/lunchbox /app/lunchbox

