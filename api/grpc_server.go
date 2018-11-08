// Package api provides the local go-spacemesh API endpoints. e.g. json-http and grpc-http2
package api

import (
	"github.com/MedRecHackathon/go-spacemesh/api/config"
	"github.com/MedRecHackathon/go-spacemesh/api/pb"
	"github.com/MedRecHackathon/go-spacemesh/log"
	"strconv"

	"net"

	"github.com/MedRecHackathon/go-spacemesh/p2p"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// SpaceMeshGrpcService is a grpc server providing the Spacemesh api
type SpaceMeshGrpcService struct {
	Server *grpc.Server
	Port   uint
	app    p2p.Service
}

// Echo returns the response for an echo api request
func (s SpaceMeshGrpcService) Echo(ctx context.Context, in *pb.SimpleMessage) (*pb.SimpleMessage, error) {
	return &pb.SimpleMessage{Value: in.Value}, nil
}

// Echo returns the response for an echo api request
func (s SpaceMeshGrpcService) RegisterProtocol(ctx context.Context, in *pb.Protocol) (*pb.SimpleMessage, error) {
	cn := s.app.RegisterProtocol(in.Name)
	destinationAddress, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(int(in.Port)))
	if err != nil {
		return nil, err
	}

	//source,err := net.ResolveUDPAddr("udp4", "127.0.0.1:1234")
	//if err != nil {
	//	return nil, err
	//}
	log.Info("Sending protocol %v to :%v", in.Name, destinationAddress)
	connection, err := net.Dial("udp4", destinationAddress.String()) //ialUDP("udp4", source, destinationAddress)
	if err != nil {
		return nil, err
	}

	go func() {
		defer connection.Close()
	Loop:
		for {
			select {
			case ime, ok := <-cn:
				if !ok {
					break Loop
				}
				log.Info("Sending message %v to %v", destinationAddress, ime.Data())
				_, err := connection.Write(ime.Data())
				if err != nil {
					log.Error("Received error : %v", err)
				}
			}
		}
	}()

	return &pb.SimpleMessage{Value: "ok"}, nil
}

// Echo returns the response for an echo api request
func (s SpaceMeshGrpcService) SendMessage(ctx context.Context, in *pb.InMessage) (*pb.SimpleMessage, error) {
	err := s.app.SendMessage(in.NodeID, in.ProtocolName, in.Payload)
	if err != nil {
		return nil, err
	}
	return &pb.SimpleMessage{Value: "ok"}, nil
}

// Echo returns the response for an echo api request
func (s SpaceMeshGrpcService) Broadcast(ctx context.Context, in *pb.BroadcastMessage) (*pb.SimpleMessage, error) {
	err := s.app.Broadcast(in.ProtocolName, in.Payload)
	if err != nil {
		log.Error("Error trying to broadcast err:", err)
		return nil, err
	}
	return &pb.SimpleMessage{Value: "ok"}, nil
}

// StopService stops the grpc service.
func (s SpaceMeshGrpcService) StopService() {
	log.Debug("Stopping grpc service...")
	s.Server.Stop()
	log.Debug("grpc service stopped...")

}

// NewGrpcService create a new grpc service using config data.
func NewGrpcService(app p2p.Service) *SpaceMeshGrpcService {
	port := config.ConfigValues.GrpcServerPort
	server := grpc.NewServer()
	return &SpaceMeshGrpcService{Server: server, Port: uint(port), app: app}
}

// StartService starts the grpc service.
func (s SpaceMeshGrpcService) StartService(status chan bool) {
	go s.startServiceInternal(status)
}

// This is a blocking method designed to be called using a go routine
func (s SpaceMeshGrpcService) startServiceInternal(status chan bool) {
	port := config.ConfigValues.GrpcServerPort
	addr := ":" + strconv.Itoa(int(port))

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("failed to listen", err)
		return
	}

	pb.RegisterSpaceMeshServiceServer(s.Server, s)

	// SubscribeOnNewConnections reflection service on gRPC server
	reflection.Register(s.Server)

	log.Debug("grpc API listening on port %d", port)

	if status != nil {
		status <- true
	}

	// start serving - this blocks until err or server is stopped
	if err := s.Server.Serve(lis); err != nil {
		log.Error("grpc stopped serving", err)
	}

	if status != nil {
		status <- true
	}

}
