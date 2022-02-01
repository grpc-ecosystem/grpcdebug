package transport

import (
	"context"
	"log"
	"time"

	csdspb "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	"github.com/grpc-ecosystem/grpcdebug/cmd/config"
	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"
	"google.golang.org/grpc"
	zpb "google.golang.org/grpc/channelz/grpc_channelz_v1"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var conn *grpc.ClientConn
var channelzClient zpb.ChannelzClient
var csdsClient csdspb.ClientStatusDiscoveryServiceClient
var healthClient healthpb.HealthClient

const connectionTimeout = time.Second * 5
const rpcTimeout = time.Second * 15

// Connect connects to the service at address and creates stubs
func Connect(c config.ServerConfig) {
	verbose.Debugf("Connecting with %v", c)
	var err error
	var credOption grpc.DialOption
	if c.CredentialFile != "" {
		cred, err := credentials.NewClientTLSFromFile(c.CredentialFile, c.ServerNameOverride)
		if err != nil {
			log.Fatalf("failed to create credential: %v", err)
		}
		credOption = grpc.WithTransportCredentials(cred)
	} else {
		credOption = grpc.WithInsecure()
	}
	// Dial, wait for READY, with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	conn, err = grpc.DialContext(ctx, c.RealAddress, credOption, grpc.WithBlock())
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	channelzClient = zpb.NewChannelzClient(conn)
	csdsClient = csdspb.NewClientStatusDiscoveryServiceClient(conn)
	healthClient = healthpb.NewHealthClient(conn)
}

// Channels returns all available channels
func Channels(startID, maxResults int64) []*zpb.Channel {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	channels, err := channelzClient.GetTopChannels(ctx, &zpb.GetTopChannelsRequest{StartChannelId: startID, MaxResults: maxResults})
	if err != nil {
		log.Fatalf("failed to fetch top channels: %v", err)
	}
	return channels.Channel
}

// Channel returns the channel with given channel ID
func Channel(channelID int64) *zpb.Channel {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	channel, err := channelzClient.GetChannel(ctx, &zpb.GetChannelRequest{ChannelId: channelID})
	if err != nil {
		log.Fatalf("failed to fetch channel id=%v: %v", channelID, err)
	}
	return channel.Channel
}

// Subchannel returns the queried subchannel
func Subchannel(subchannelID int64) *zpb.Subchannel {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	subchannel, err := channelzClient.GetSubchannel(ctx, &zpb.GetSubchannelRequest{SubchannelId: subchannelID})
	if err != nil {
		log.Fatalf("failed to fetch subchannel (id=%v): %v", subchannelID, err)
	}
	return subchannel.Subchannel
}

// Servers returns all available servers
func Servers(startID, maxResults int64) []*zpb.Server {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	servers, err := channelzClient.GetServers(ctx, &zpb.GetServersRequest{StartServerId: startID, MaxResults: maxResults})
	if err != nil {
		log.Fatalf("failed to fetch servers: %v", err)
	}
	return servers.Server
}

// Server returns a server
func Server(serverID int64) *zpb.Server {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	server, err := channelzClient.GetServer(ctx, &zpb.GetServerRequest{ServerId: serverID})
	if err != nil {
		log.Fatalf("failed to fetch server (id=%v): %v", serverID, err)
	}
	return server.Server
}

// Socket returns a socket
func Socket(socketID int64) *zpb.Socket {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	socket, err := channelzClient.GetSocket(ctx, &zpb.GetSocketRequest{SocketId: socketID})
	if err != nil {
		log.Fatalf("failed to fetch socket (id=%v): %v", socketID, err)
	}
	return socket.Socket
}

// ServerSocket returns all sockets of this server
func ServerSocket(serverID, startID, maxResults int64) []*zpb.Socket {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var s []*zpb.Socket
	serverSocketResp, err := channelzClient.GetServerSockets(
		ctx,
		&zpb.GetServerSocketsRequest{
			ServerId:      serverID,
			StartSocketId: startID,
			MaxResults:    maxResults,
		},
	)
	if err != nil {
		log.Fatalf("failed to fetch server sockets (id=%v): %v", serverID, err)
	}
	for _, socketRef := range serverSocketResp.SocketRef {
		s = append(s, Socket(socketRef.SocketId))
	}
	return s
}

// FetchClientStatus fetches the xDS resources status
func FetchClientStatus() *csdspb.ClientStatusResponse {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := csdsClient.FetchClientStatus(ctx, &csdspb.ClientStatusRequest{})
	if err != nil {
		log.Fatalf("failed to fetch xds config: %v", err)
	}
	return resp
}

// GetHealthStatus fetches the health checking status of the service from peer
func GetHealthStatus(service string) string {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		verbose.Debugf("failed to fetch health status for \"%s\": %v", service, err)
		return healthpb.HealthCheckResponse_SERVICE_UNKNOWN.String()
	}
	return resp.Status.String()
}
