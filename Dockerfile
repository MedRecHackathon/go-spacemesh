FROM golang:1.9.2-alpine3.6 AS build
ARG BRANCH=master
RUN apk add --no-cache make git

RUN go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
RUN go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
RUN go get -u github.com/golang/protobuf/protoc-gen-go
RUN go get -u github.com/kardianos/govendor
RUN echo ${BRANCH}
RUN mkdir -p src/github.com/spacemeshos; cd src/github.com/spacemeshos; git clone https://github.com/MedRecHackathon/go-spacemesh; cd go-spacemesh; git checkout ${BRANCH}; go build; govendor sync; make
RUN cp /go/src/github.com/MedRecHackathon/go-spacemesh/config.toml /go

ENTRYPOINT /go/src/github.com/MedRecHackathon/go-spacemesh/go-spacemesh $BOOTPARAMS

EXPOSE 7513
EXPOSE 9090
EXPOSE 9091
