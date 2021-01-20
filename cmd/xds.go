package cmd

import (
	"fmt"
	"strings"

	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"

	adminpb "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	csdspb "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	"github.com/golang/protobuf/ptypes"
	timestamppb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func printJson(m proto.Message) error {
	var option protojson.MarshalOptions
	option.Multiline = true
	option.Indent = "  "
	option.UseProtoNames = false
	option.UseEnumNumbers = false
	jsonbytes, err := option.Marshal(m)
	if err != nil {
		return err
	}
	fmt.Println(string(jsonbytes))
	return nil
}

func sortPerXdsConfigs(clientStatus *csdspb.ClientStatusResponse) {
	var xdsConfigs [4]*csdspb.PerXdsConfig
	for _, xdsConfig := range clientStatus.Config[0].XdsConfig {
		switch xdsConfig.PerXdsConfig.(type) {
		case *csdspb.PerXdsConfig_ListenerConfig:
			xdsConfigs[0] = xdsConfig
		case *csdspb.PerXdsConfig_RouteConfig:
			xdsConfigs[1] = xdsConfig
		case *csdspb.PerXdsConfig_ClusterConfig:
			xdsConfigs[2] = xdsConfig
		case *csdspb.PerXdsConfig_EndpointConfig:
			xdsConfigs[3] = xdsConfig
		}
	}
	clientStatus.Config[0].XdsConfig = xdsConfigs[:]
}

func xdsConfigCommandRunWithError(cmd *cobra.Command, args []string) error {
	clientStatus := transport.FetchClientStatus()
	if len(args) == 0 {
		sortPerXdsConfigs(clientStatus)
		return printJson(clientStatus)
	}
	// Filter the CSDS output
	var demand string
	demand = strings.ToLower(args[0])
	for _, xdsConfig := range clientStatus.Config[0].XdsConfig {
		switch xdsConfig.PerXdsConfig.(type) {
		case *csdspb.PerXdsConfig_ListenerConfig:
			if demand == "lds" {
				return printJson(xdsConfig.GetListenerConfig())
			}
		case *csdspb.PerXdsConfig_RouteConfig:
			if demand == "rds" {
				return printJson(xdsConfig.GetRouteConfig())
			}
		case *csdspb.PerXdsConfig_ClusterConfig:
			if demand == "cds" {
				return printJson(xdsConfig.GetClusterConfig())
			}
		case *csdspb.PerXdsConfig_EndpointConfig:
			if demand == "eds" {
				return printJson(xdsConfig.GetEndpointConfig())
			}
		}
	}
	return fmt.Errorf("Failed to find xDS config with type %s", args[0])
}

var xdsConfigCmd = &cobra.Command{
	Use:   "config [lds|rds|cds|eds]",
	Short: "Dump the operating xDS configs.",
	RunE:  xdsConfigCommandRunWithError,
	Args:  cobra.MaximumNArgs(1),
}

type xdsResourceStatusEntry struct {
	Name        string
	Status      adminpb.ClientResourceStatus
	Version     string
	Type        string
	LastUpdated *timestamppb.Timestamp
}

func prettyClientResourceStatus(s adminpb.ClientResourceStatus) string {
	return adminpb.ClientResourceStatus_name[int32(s)]
}

func printStatusEntry(entry *xdsResourceStatusEntry) {
	fmt.Fprintf(
		w, "%v\t%v\t%v\t%v\t%v\t\n",
		entry.Name,
		prettyClientResourceStatus(entry.Status),
		entry.Version,
		entry.Type,
		prettyTime(entry.LastUpdated),
	)
}

func xdsStatusCommandRunWithError(cmd *cobra.Command, args []string) error {
	clientStatus := transport.FetchClientStatus()

	fmt.Fprintln(w, "Name\tStatus\tVersion\tType\tLastUpdated")
	config := clientStatus.Config[0]
	for _, xdsConfig := range config.XdsConfig {
		switch xdsConfig.PerXdsConfig.(type) {
		case *csdspb.PerXdsConfig_ListenerConfig:
			for _, dynamicListener := range xdsConfig.GetListenerConfig().DynamicListeners {
				var entry = xdsResourceStatusEntry{
					Name:   dynamicListener.Name,
					Status: dynamicListener.ClientStatus,
				}
				if state := dynamicListener.GetActiveState(); state != nil {
					entry.Version = state.VersionInfo
					entry.Type = state.Listener.TypeUrl
					entry.LastUpdated = state.LastUpdated
				}
				printStatusEntry(&entry)
			}
		case *csdspb.PerXdsConfig_RouteConfig:
			for _, dynamicRouteConfig := range xdsConfig.GetRouteConfig().DynamicRouteConfigs {
				var entry = xdsResourceStatusEntry{
					Status:      dynamicRouteConfig.ClientStatus,
					Version:     dynamicRouteConfig.VersionInfo,
					Type:        dynamicRouteConfig.RouteConfig.TypeUrl,
					LastUpdated: dynamicRouteConfig.LastUpdated,
				}
				if packed := dynamicRouteConfig.GetRouteConfig(); packed != nil {
					var routeConfig routepb.RouteConfiguration
					if err := ptypes.UnmarshalAny(packed, &routeConfig); err != nil {
						return err
					}
					entry.Name = routeConfig.Name
				}
				printStatusEntry(&entry)
			}
		case *csdspb.PerXdsConfig_ClusterConfig:
			for _, dynamicCluster := range xdsConfig.GetClusterConfig().DynamicActiveClusters {
				var entry = xdsResourceStatusEntry{
					Status:      dynamicCluster.ClientStatus,
					Version:     dynamicCluster.VersionInfo,
					Type:        dynamicCluster.Cluster.TypeUrl,
					LastUpdated: dynamicCluster.LastUpdated,
				}
				if packed := dynamicCluster.GetCluster(); packed != nil {
					var cluster clusterpb.Cluster
					if err := ptypes.UnmarshalAny(packed, &cluster); err != nil {
						return err
					}
					entry.Name = cluster.Name
				}
				printStatusEntry(&entry)
			}
		case *csdspb.PerXdsConfig_EndpointConfig:
			for _, dynamicEndpoint := range xdsConfig.GetEndpointConfig().GetDynamicEndpointConfigs() {
				var entry = xdsResourceStatusEntry{
					Status:      dynamicEndpoint.ClientStatus,
					Version:     dynamicEndpoint.VersionInfo,
					Type:        dynamicEndpoint.EndpointConfig.TypeUrl,
					LastUpdated: dynamicEndpoint.LastUpdated,
				}
				if packed := dynamicEndpoint.GetEndpointConfig(); packed != nil {
					var endpoint endpointpb.ClusterLoadAssignment
					if err := ptypes.UnmarshalAny(packed, &endpoint); err != nil {
						return err
					}
					entry.Name = endpoint.ClusterName
				}
				printStatusEntry(&entry)
			}
		}
	}
	w.Flush()
	return nil
}

var xdsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the config synchronization status.",
	RunE:  xdsStatusCommandRunWithError,
}

var xdsCmd = &cobra.Command{
	Use:   "xds",
	Short: "Fetch xDS related information.",
}

func init() {
	xdsCmd.AddCommand(xdsConfigCmd)
	xdsCmd.AddCommand(xdsStatusCmd)
	rootCmd.AddCommand(xdsCmd)
}
