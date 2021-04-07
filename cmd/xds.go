package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"
	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"

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

func printJSON(m proto.Message) error {
	option := protojson.MarshalOptions{
		Multiline:      true,
		Indent:         "  ",
		UseProtoNames:  false,
		UseEnumNumbers: false,
	}
	jsonbytes, err := option.Marshal(m)
	if err != nil {
		return err
	}
	fmt.Println(string(jsonbytes))
	return nil
}

func priorityPerXdsConfig(x *csdspb.PerXdsConfig) int {
	switch x.PerXdsConfig.(type) {
	case *csdspb.PerXdsConfig_ListenerConfig:
		return 0
	case *csdspb.PerXdsConfig_RouteConfig:
		return 1
	case *csdspb.PerXdsConfig_ClusterConfig:
		return 2
	case *csdspb.PerXdsConfig_EndpointConfig:
		return 3
	default:
		return 4
	}
}

func sortPerXdsConfigs(clientStatus *csdspb.ClientStatusResponse) {
	var xdsConfigs []*csdspb.PerXdsConfig = clientStatus.Config[0].XdsConfig
	sort.Slice(xdsConfigs, func(i, j int) bool {
		return priorityPerXdsConfig(xdsConfigs[i]) < priorityPerXdsConfig(xdsConfigs[j])
	})
	clientStatus.Config[0].XdsConfig = xdsConfigs
}

func xdsConfigCommandRunWithError(cmd *cobra.Command, args []string) error {
	clientStatus := transport.FetchClientStatus()
	if len(clientStatus.Config) != 1 {
		return fmt.Errorf("Unexpected number of ClientConfig %v", len(clientStatus.Config))
	}
	if len(args) == 0 {
		sortPerXdsConfigs(clientStatus)
		return printJSON(clientStatus)
	}
	// Filter the CSDS output
	var demand string
	demand = strings.ToLower(args[0])
	for _, xdsConfig := range clientStatus.Config[0].XdsConfig {
		switch xdsConfig.PerXdsConfig.(type) {
		case *csdspb.PerXdsConfig_ListenerConfig:
			if demand == "lds" {
				return printJSON(xdsConfig.GetListenerConfig())
			}
		case *csdspb.PerXdsConfig_RouteConfig:
			if demand == "rds" {
				return printJSON(xdsConfig.GetRouteConfig())
			}
		case *csdspb.PerXdsConfig_ClusterConfig:
			if demand == "cds" {
				return printJSON(xdsConfig.GetClusterConfig())
			}
		case *csdspb.PerXdsConfig_EndpointConfig:
			if demand == "eds" {
				return printJSON(xdsConfig.GetEndpointConfig())
			}
		}
	}
	verbose.Debugf("Failed to find xDS config with type %s", args[0])
	return nil
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

func printStatusEntry(entry *xdsResourceStatusEntry) {
	fmt.Fprintf(
		w, "%v\t%v\t%v\t%v\t%v\t\n",
		entry.Name,
		entry.Status,
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
				entry := xdsResourceStatusEntry{
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
				entry := xdsResourceStatusEntry{
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
				entry := xdsResourceStatusEntry{
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
				entry := xdsResourceStatusEntry{
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
