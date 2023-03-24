FROM golang:latest as build
WORKDIR /go/src/app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/rotation-detector

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/rotation-detector .
CMD ["/rotation-detector"]
