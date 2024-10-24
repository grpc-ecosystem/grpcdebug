package cmd

import (
	"fmt"
	"sort"
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

var xdsTypeFlag string

func printProtoBufMessageAsJSON(m proto.Message) error {
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
	for _, config := range clientStatus.Config {
		sort.Slice(config.XdsConfig, func(i, j int) bool {
			return priorityPerXdsConfig(config.XdsConfig[i]) < priorityPerXdsConfig(config.XdsConfig[j])
		})
	}
}

func xdsConfigCommandRunWithError(cmd *cobra.Command, args []string) error {
	clientStatus := transport.FetchClientStatus()
	fmt.Printf("Received %v ClientConfig(s)\n", len(clientStatus.Config))
	if xdsTypeFlag == "" {
		// No filters, just print the whole thing
		sortPerXdsConfigs(clientStatus)
		return printProtoBufMessageAsJSON(clientStatus)
	}
	// Parse flags
	wantXdsTypes := strings.Split(xdsTypeFlag, ",")
	var wantLDS, wantRDS, wantCDS, wantEDS bool
	for _, wantXdsType := range wantXdsTypes {
		switch strings.ToLower(wantXdsType) {
		case "lds":
			wantLDS = true
		case "rds":
			wantRDS = true
		case "cds":
			wantCDS = true
		case "eds":
			wantEDS = true
		}
	}
	for idx, config := range clientStatus.Config {
		fmt.Printf("\n=== Config %d ===\n", idx)
		// Filter the CSDS output
		for _, genericXdsConfig := range config.GenericXdsConfigs {
			var printSubject proto.Message
			tokens := strings.Split(genericXdsConfig.TypeUrl, ".")
			switch tokens[len(tokens)-1] {
			case "Listener":
				if wantLDS {
					printSubject = genericXdsConfig.GetXdsConfig()
				}
			case "RouteConfiguration":
				if wantRDS {
					printSubject = genericXdsConfig.GetXdsConfig()
				}
			case "Cluster":
				if wantCDS {
					printSubject = genericXdsConfig.GetXdsConfig()
				}
			case "ClusterLoadAssignment":
				if wantEDS {
					printSubject = genericXdsConfig.GetXdsConfig()
				}
			}
			if printSubject != nil {
				err := printProtoBufMessageAsJSON(printSubject)
				if err != nil {
					return fmt.Errorf("Failed to print xDS config: %v", err)
				}
			}
		}
		if len(config.GenericXdsConfigs) == 0 {
			for _, xdsConfig := range config.XdsConfig {
				var printSubject proto.Message
				switch xdsConfig.PerXdsConfig.(type) {
				case *csdspb.PerXdsConfig_ListenerConfig:
					if wantLDS {
						printSubject = xdsConfig.GetListenerConfig()
					}
				case *csdspb.PerXdsConfig_RouteConfig:
					if wantRDS {
						printSubject = xdsConfig.GetRouteConfig()
					}
				case *csdspb.PerXdsConfig_ClusterConfig:
					if wantCDS {
						printSubject = xdsConfig.GetClusterConfig()
					}
				case *csdspb.PerXdsConfig_EndpointConfig:
					if wantEDS {
						printSubject = xdsConfig.GetEndpointConfig()
					}
				}
				if printSubject != nil {
					err := printProtoBufMessageAsJSON(printSubject)
					if err != nil {
						return fmt.Errorf("Failed to print xDS config: %v", err)
					}
				}
			}
		}
	}
	return nil
}

var xdsConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Dump the operating xDS configs.",
	RunE:  xdsConfigCommandRunWithError,
	Args:  cobra.NoArgs,
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
	fmt.Printf("Received %v ClientConfig(s)\n", len(clientStatus.Config))

	for idx, config := range clientStatus.Config {
		fmt.Fprintf(w, "\n=== Config %d ===\n", idx)
		fmt.Fprintln(w, "Name\tStatus\tVersion\tType\tLastUpdated")
		for _, genericXdsConfig := range config.GenericXdsConfigs {
			entry := xdsResourceStatusEntry{
				Name:        genericXdsConfig.Name,
				Status:      genericXdsConfig.ClientStatus,
				Version:     genericXdsConfig.VersionInfo,
				Type:        genericXdsConfig.TypeUrl,
				LastUpdated: genericXdsConfig.LastUpdated,
			}
			printStatusEntry(&entry)
		}
		if len(config.GenericXdsConfigs) == 0 {
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
		}
		w.Flush()
	}
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
	xdsConfigCmd.Flags().StringVarP(&xdsTypeFlag, "type", "y", "", "Filters the wanted type of xDS config to print (separated by commas) (available types: LDS,RDS,CDS,EDS) (by default, print all)")
	xdsCmd.AddCommand(xdsConfigCmd)
	xdsCmd.AddCommand(xdsStatusCmd)
	rootCmd.AddCommand(xdsCmd)
}
