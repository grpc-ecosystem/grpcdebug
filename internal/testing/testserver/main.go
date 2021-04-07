// Testserver mocking the responses of Channelz/CSDS/Health
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/testdata"

	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	csdspb "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	servingPortFlag     = flag.Int("serving", 10001, "the serving port")
	adminPortFlag       = flag.Int("admin", 50051, "the admin port")
	secureAdminPortFlag = flag.Int("secure_admin", 50052, "the secure admin port")
	healthFlag          = flag.Bool("health", true, "the health checking status")
	qpsFlag             = flag.Int("qps", 10, "The size of the generated load against itself")
	abortPercentageFlag = flag.Int("abort_percentage", 10, "The percentage of failed RPCs")
)

// Prepare the CSDS response
var csdsResponse csdspb.ClientStatusResponse

func init() {
	file, err := os.Open("csds_config_dump.json")
	if err != nil {
		panic(err)
	}
	configDump, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	if err := protojson.Unmarshal([]byte(configDump), &csdsResponse); err != nil {
		panic(err)
	}
}

// Implements the Greeter service
type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {

	if int(rand.Int31n(100)) <= *abortPercentageFlag {
		return nil, grpc.Errorf(codes.Code(rand.Int31n(15)+1), "Fault injected")
	}
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

// Implements the CSDS service
type mockCsdsServer struct {
	csdspb.UnimplementedClientStatusDiscoveryServiceServer
}

func (*mockCsdsServer) FetchClientStatus(ctx context.Context, req *csdspb.ClientStatusRequest) (*csdspb.ClientStatusResponse, error) {
	return &csdsResponse, nil
}

func setupAdminServer(s *grpc.Server) {
	reflection.Register(s)
	service.RegisterChannelzServiceToServer(s)
	csdspb.RegisterClientStatusDiscoveryServiceServer(s, &mockCsdsServer{})
	healthcheck := health.NewServer()
	if *healthFlag {
		healthcheck.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		healthcheck.SetServingStatus("helloworld.Greeter", healthpb.HealthCheckResponse_SERVING)
	} else {
		healthcheck.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		healthcheck.SetServingStatus("helloworld.Greeter", healthpb.HealthCheckResponse_NOT_SERVING)
	}
	healthpb.RegisterHealthServer(s, healthcheck)
}

func main() {
	// Parse the flags
	flag.Parse()
	// Seed the RNG
	rand.Seed(time.Now().UnixNano())
	// Creates the primary server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *servingPortFlag))
	if err != nil {
		panic(err)
	}
	defer lis.Close()
	fmt.Printf("Serving Business Logic on :%d\n", *servingPortFlag)
	cert, err := tls.LoadX509KeyPair(testdata.Path("server1.pem"), testdata.Path("server1.key"))
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}
	s := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	pb.RegisterGreeterServer(s, &server{})
	go s.Serve(lis)
	defer s.Stop()
	// Creates the admin server without credentials
	insecureListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *adminPortFlag))
	if err != nil {
		panic(err)
	}
	defer insecureListener.Close()
	insecureAdminServer := grpc.NewServer()
	setupAdminServer(insecureAdminServer)
	go insecureAdminServer.Serve(insecureListener)
	defer insecureAdminServer.Stop()
	fmt.Printf("Serving Insecure Admin Services on :%d\n", *adminPortFlag)
	// Creates the admin server with credentials
	secureListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *secureAdminPortFlag))
	if err != nil {
		panic(err)
	}
	defer secureListener.Close()
	secureAdminServer := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	setupAdminServer(secureAdminServer)
	go secureAdminServer.Serve(secureListener)
	defer secureAdminServer.Stop()
	fmt.Printf("Serving Secure Admin Services on :%d\n", *secureAdminPortFlag)
	// Creates a client to hydrate the primary server
	creds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), "*.test.youtube.com")
	if err != nil {
		panic(err)
	}
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", *servingPortFlag), grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	greeterClient := pb.NewGreeterClient(conn)
	for {
		greeterClient.SayHello(context.Background(), &pb.HelloRequest{Name: "world"})
		time.Sleep(time.Second / time.Duration(*qpsFlag))
	}
}
