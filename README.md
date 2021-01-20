# grpcdebug
[![Go Report Card](https://goreportcard.com/badge/github.com/grpc-ecosystem/grpcdebug)](https://goreportcard.com/report/github.com/grpc-ecosystem/grpcdebug)

grpcdebug is a command line interface focusing on simplifying the debugging
process of gRPC applications. grpcdebug fetches the internal states of the gRPC
library from the application via gRPC protocol and provide a human-friendly UX
to browse them. Currently, it supports Channelz/Health Checking/CSDS (aka. admin
services). In other words, it can fetch statistics about how many RPCs has being
sent or failed on a given gRPC channel, it can inspect address resolution
results, it can dump the in-effective xDS configuration that directs the routing
of RPCs.

If you are looking for a tool to send gRPC requests and interact with a gRPC
server, please checkout https://github.com/fullstorydev/grpcurl.

```
grpcdebug is an gRPC service admin CLI

Usage:
  grpcdebug <target address> [flags] <command>

Available Commands:
  channelz    Display gRPC states in human readable way.
  health      Check health status of the target service (default "").
  help        Help about any command
  xds         Fetch xDS related information.

Flags:
      --credential_file string        Sets the path of the credential file; used in [tls] mode
  -h, --help                          help for grpcdebug
      --security string               Defines the type of credentials to use [tls, google-default, insecure] (default "insecure")
      --server_name_override string   Overrides the peer server name if non empty; used in [tls] mode
  -t, --timestamp                     Print timestamp as RFC3339 instead of human readable strings
  -v, --verbose                       Print verbose information for debugging

Use "grpcdebug <target address>  [command] --help" for more information about a command.
```

## Table of Contents
- [grpcdebug](#grpcdebug)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
    - [Connect & Security](#connect--security)
      - [Insecure Connection](#insecure-connection)
      - [TLS Connection - Flags](#tls-connection---flags)
      - [Server Connection Config](#server-connection-config)
    - [Health](#health)
    - [Channelz](#channelz)
      - [Usage 1: Raw Channelz Output](#usage-1-raw-channelz-output)
      - [Usage 2: List Client Channels](#usage-2-list-client-channels)
      - [Usage 3: List Servers](#usage-3-list-servers)
      - [Usage 4: Inspect a Channel](#usage-4-inspect-a-channel)
      - [Usage 5: Inspect a Subchannel](#usage-5-inspect-a-subchannel)
      - [Usage 6: Inspect a Socket](#usage-6-inspect-a-socket)
      - [Usage 7: Inspect a Server](#usage-7-inspect-a-server)
    - [Debug xDS](#debug-xds)
      - [Usage 1: xDS Resources Overview](#usage-1-xds-resources-overview)
      - [Usage 2: Dump xDS Configs](#usage-2-dump-xds-configs)
      - [Usage 3: Filter xDS Configs](#usage-3-filter-xds-configs)
  - [Admin Services](#admin-services)
    - [gRPC Java:](#grpc-java)
    - [gRPC Go:](#grpc-go)
    - [gRPC C++:](#grpc-c)

## Installation

Minimum Golang Version 1.12. Official Golang install guide: https://golang.org/doc/install.

```shell
go get github.com/grpc-ecosystem/grpcdebug
```

## Quick Start

If certain commands if confusing, please try to use `-h` to get more context. Suggestions and ideas are welcome, please post them to https://github.com/grpc-ecosystem/grpcdebug/issues!

If you haven't got your gRPC application instrumented, feel free to try out the mocking `testserver` which implemented admin services.

```shell
cd internal/testing/testserver
go run main.go
# Serving Business Logic on :10001
# Serving Insecure Admin Services on :50051
# Serving Secure Admin Services on :50052
# ...
```

### Connect & Security

#### Insecure Connection

To connect to a gRPC endpoint without any credentials, we don't any special
flags. If the local network can connect to the given gRPC endpoint, it should
just work. For example, if I have a gRPC application exposing admin services at
`localhost:50051`:

```shell
grpcdebug localhost:50051 channelz channels
```

#### TLS Connection - Flags

One way to establish a TLS connection with grpcdebug is specifying the credentials via command line flags. For example:

```shell
grpcdebug localhost:50052 --security=tls --credential_file=./internal/testing/ca.pem --server_name_override="*.test.youtube.com" channelz channels
```

#### Server Connection Config

Alternatively, like OpenSSH clients, you can specify the security settings in a
`grpcdebug_config` file. grpcdebug CLI will find matching connection config and
then use it to connect.

```
Server <Matching Pattern>
  RealAddress        <Real Target Address>
  Security           <Security Mode>
  CredentialFile     <Path to Credential File>
  ServerNameOverride <Overriding Server Name>
```

Here is an example config file [grpcdebug_config](https://github.com/grpc-ecosystem/grpcdebug/blob/main/internal/testing/grpcdebug_config).

Each server config can have following settings:

* Pattern: the string right after `Server ` which dictates if this rule should apply;
* RealAddress: if present, override the given target address, which allows giving nicknames/aliases to frequently used addresses;
* Security: allows `insecure` or `tls`, expecting more in future;
* CredentialFile: path to the credential file;
* ServerNameOverride: override the hostname, useful for local reproductions to comply the certificates' common name requirement.

grpcdebug searches the config file in following order:

1. Check if environment variable `GRPCDEBUG_CONFIG` is set, if so, load from the given path;
2. Try to load the `grpcdebug_config` file in current working directory;
3. Try to load the `grpcdebug_config` file in the user config directory (Linux: `$HOME/.config`, macOS: `$HOME/Library/Application Support`, Windows: `%AppData%`, see [`os.UserConfigDir()`](https://golang.org/pkg/os/#UserConfigDir)).

For example, we can connect to our mock test server's secure admin port via:

```shell
GRPCDEBUG_CONFIG=internal/testing/grpcdebug_config grpcdebug localhost:50052 channelz channels
# Or
GRPCDEBUG_CONFIG=internal/testing/grpcdebug_config grpcdebug prod channelz channels
```

### Health

grpcdebug can be used to fetch the health checking status of peer gRPC
application (see
[health.proto](https://github.com/grpc/grpc/blob/master/src/proto/grpc/health/v1/health.proto)).
gRPC's health checking works at service-level, meaning services registered on
the same gRPC server may have different health status. The health status of
service "" is used to represent the overall health status of the gRPC
application.

To simply fetch the overall health status:

```shell
grpcdebug localhost:50051 health
# SERVING
# or
# NOT_SERVING
```

Or fetch individual service's health status:

```shell
grpcdebug localhost:50051 health "" helloworld.Greeter
# :                     SERVING
# helloworld.Greeter:   SERVING
```

### Channelz

Channelz is a channel tracing library that allows applications to remotely query
gRPC internal debug information. Also, Channelz has a web interface (see
[gdebug](https://github.com/grpc/grpc-experiments/tree/master/gdebug)).
grpcdebug is able to fetch information and present it in a more readable way.


#### Usage 1: Raw Channelz Output

For all Channelz commands, you can add `--json` to get the raw Channelz output.

```shell
grpcdebug localhost:50051 channelz channels --json
grpcdebug localhost:50051 channelz servers --json
```

#### Usage 2: List Client Channels

```shell
grpcdebug localhost:50051 channelz channels
# Channel ID   Target            State     Calls(Started/Succeeded/Failed)   Created Time
# 7            localhost:10001   READY     5136/4631/505                     8 minutes ago
```

#### Usage 3: List Servers

```shell
grpcdebug localhost:50051 channelz servers
# Server ID   Listen Addresses   Calls(Started/Succeeded/Failed)   Last Call Started
# 1           [:::10001]         2852/2530/322                     now
# 2           [:::50051]         29/28/0                           now
# 3           [:::50052]         4/4/0                             26 seconds ago
```

#### Usage 4: Inspect a Channel

You can identify a channel via the Channel ID or a URL matching its target (if multiple hit, return first match).

```shell
grpcdebug localhost:50051 channelz channel localhost:10001
grpcdebug localhost:50051 channelz channel 7
# Channel ID:        7
# Target:            localhost:10001
# State:             READY
# Calls Started:     3976
# Calls Succeeded:   3520
# Calls Failed:      456
# Created Time:      6 minutes ago
# ---
# Subchannel ID   Target            State     Calls(Started/Succeeded/Failed)   CreatedTime
# 8               localhost:10001   READY     3976/3520/456                     6 minutes ago
# ---
# Severity   Time            Child Ref                      Description
# CT_INFO    6 minutes ago                                  Channel Created
# CT_INFO    6 minutes ago                                  Resolver state updated: {Addresses:[{Addr:localhost:10001 ServerName: Attributes:<nil> Type:0 Metadata:<nil>}] ServiceConfig:<nil> Attributes:<nil>} (resolver returned new addresses)
# CT_INFO    6 minutes ago                                  Channel switches to new LB policy "pick_first"
# CT_INFO    6 minutes ago   subchannel(subchannel_id:8 )   Subchannel(id:8) created
# CT_INFO    6 minutes ago                                  Channel Connectivity change to CONNECTING
# CT_INFO    6 minutes ago                                  Channel Connectivity change to READY
```

#### Usage 5: Inspect a Subchannel

```shell
grpcdebug localhost:50051 channelz subchannel 8
# Subchannel ID:     8
# Target:            localhost:10001
# State:             READY
# Calls Started:     4490
# Calls Succeeded:   3966
# Calls Failed:      524
# Created Time:      7 minutes ago
# ---
# Socket ID   Local->Remote          Streams(Started/Succeeded/Failed)   Messages(Sent/Received)
# 9           ::1:47436->::1:10001   4490/4490/0                         4490/3966
```

#### Usage 6: Inspect a Socket

```shell
grpcdebug localhost:50051 channelz socket 9
# Socket ID:                       9
# Address:                         ::1:47436->::1:10001
# Streams Started:                 4807
# Streams Succeeded:               4807
# Streams Failed:                  0
# Messages Sent:                   4807
# Messages Received:               4243
# Keep Alives Sent:                0
# Last Local Stream Created:       now
# Last Remote Stream Created:      a long while ago
# Last Message Sent Created:       now
# Last Message Received Created:   now
# Local Flow Control Window:       65535
# Remote Flow Control Window:      65535
# ---
# Socket Options Name   Value
# SO_LINGER             [type.googleapis.com/grpc.channelz.v1.SocketOptionLinger]:{duration:{}}
# SO_RCVTIMEO           [type.googleapis.com/grpc.channelz.v1.SocketOptionTimeout]:{duration:{}}
# SO_SNDTIMEO           [type.googleapis.com/grpc.channelz.v1.SocketOptionTimeout]:{duration:{}}
# TCP_INFO              [type.googleapis.com/grpc.channelz.v1.SocketOptionTcpInfo]:{tcpi_state:1  tcpi_options:7  tcpi_rto:204000  tcpi_ato:40000  tcpi_snd_mss:32768  tcpi_rcv_mss:1093  tcpi_last_data_sent:16  tcpi_last_data_recv:16  tcpi_last_ack_recv:16  tcpi_pmtu:65536  tcpi_rcv_ssthresh:65476  tcpi_rtt:192  tcpi_rttvar:153  tcpi_snd_ssthresh:2147483647  tcpi_snd_cwnd:10  tcpi_advmss:65464  tcpi_reordering:3}
# ---
# Security Model:   TLS
# Standard Name:    TLS_AES_128_GCM_SHA256
```

#### Usage 7: Inspect a Server

```shell
grpcdebug localhost:50051 channelz server 1
# Server Id:           1
# Listen Addresses:    [:::10001]
# Calls Started:       5250
# Calls Succeeded:     4647
# Calls Failed:        603
# Last Call Started:   now
# ---
# Socket ID   Local->Remote          Streams(Started/Succeeded/Failed)   Messages(Sent/Received)
# 10          ::1:10001->::1:47436   5250/5250/0                         4647/5250
```

### Debug xDS

[xDS](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/dynamic_configuration)
is a data plane configuration API commonly used in service mesh projects. It's
created by Envoy, used by Istio, Traffic Director, and gRPC.

#### Usage 1: xDS Resources Overview

The xDS resources status might be REQUESTED/DOES_NOT_EXIST/ACKED/NACKED (see [config_dump.proto](https://github.com/envoyproxy/envoy/blob/b0ce15c96cebd89cf391869e49017325cd7faaa8/api/envoy/admin/v3/config_dump.proto#L22)). This view is intended for a quick scan if a configuration is propagated from the service mesh control plane.

```shell
grpcdebug localhost:50051 xds status
# Name                                                                   Status    Version               Type                                                                 LastUpdated
# xds-test-server:1337                                                   ACKED     1617141154495058478   type.googleapis.com/envoy.config.listener.v3.Listener                2 days ago
# URL_MAP/1040920224690_sergii-psm-test-url-map_0_xds-test-server:1337   ACKED     1617141154495058478   type.googleapis.com/envoy.config.route.v3.RouteConfiguration         2 days ago
# cloud-internal-istio:cloud_mp_1040920224690_6530603179561593229        ACKED     1617141154495058478   type.googleapis.com/envoy.config.cluster.v3.Cluster                  2 days ago
# cloud-internal-istio:cloud_mp_1040920224690_6530603179561593229        ACKED     1                     type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment   2 days ago
```

#### Usage 2: Dump xDS Configs

```shell
grpcdebug localhost:50051 xds config
# {
#   "config":  [
#     {
#       "node":  {
#         "id":  "projects/1040920224690/networks/default/nodes/5cc9170c-d5b4-4061-b431-c1d43e6ac0ab",
#         "cluster":  "cluster",
#         "metadata":  {
#           "INSTANCE_IP":  "192.168.120.31",
#           "TRAFFICDIRECTOR_GCP_PROJECT_NUMBER":  "1040920224690",
#           "TRAFFICDIRECTOR_NETWORK_NAME":  "default"
#         },
# ...
```

For an example config dump, see [csds_config_dump.json](https://github.com/grpc-ecosystem/grpcdebug/blob/main/internal/testing/testserver/csds_config_dump.json).

#### Usage 3: Filter xDS Configs

The dumped xDS config can be quite verbose, if I only interested in certain xDS type, grpcdebug can only print the selected section.

```shell
grpcdebug localhost:50051 xds config eds
# {
#   "dynamicEndpointConfigs":  [
#     {
#       "versionInfo":  "1",
#       "endpointConfig":  {
#         "@type":  "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment",
#         "clusterName":  "cloud-internal-istio:cloud_mp_1040920224690_6530603179561593229",
#         "endpoints":  [
#           {
#             "locality":  {
#               "subZone":  "jf:us-central1-a_7062512536751318190_neg"
#             },
#             "lbEndpoints":  [
#               {
#                 "endpoint":  {
#                   "address":  {
#                     "socketAddress":  {
#                       "address":  "192.168.120.26",
#                       "portValue":  8080
#                     }
#                   }
#                 },
#                 "healthStatus":  "HEALTHY"
#               }
#             ],
#             "loadBalancingWeight":  100
#           }
#         ]
#       },
#       "lastUpdated":  "2021-03-31T01:20:33.936Z",
#       "clientStatus":  "ACKED"
#     }
#   ]
# }
```

## Admin Services

### gRPC Java:

```java
server = ServerBuilder.forPort(50051)
        .useTransportSecurity(certChainFile, privateKeyFile)
        .addServices(AdminInterface.getStandardServices())
        .build()
        .start();
server.awaitTermination();
```


### gRPC Go:

```golang
lis, err := net.Listen("tcp", ":50051")
if err != nil {
        log.Fatalf("failed to listen: %v", err)
}
defer lis.Close()
grpcServer := grpc.NewServer(...opts)
adminServices.RegisterAdminServicesToServer(grpcServer)
if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
}
```


### gRPC C++:

```cpp
grpc::ServerBuilder builder;
grpc::AddAdminServices(&builder);
builder.AddListeningPort(":50051", grpc::ServerCredentials(...));
std::unique_ptr<grpc::Server> server(builder.BuildAndStart());
```
