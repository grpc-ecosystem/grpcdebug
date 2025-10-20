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

var (
	xdsTypeFlag  string
	xdsScopeFlag string
)

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
	for _, cfg := range clientStatus.Config {
		sort.Slice(cfg.XdsConfig, func(i, j int) bool {
			return priorityPerXdsConfig(cfg.XdsConfig[i]) < priorityPerXdsConfig(cfg.XdsConfig[j])
		})
	}
}

func filterConfigsByScope(all []*csdspb.ClientConfig, scope string) ([]*csdspb.ClientConfig, error) {
	if scope == "" {
		return all, nil
	}
	var out []*csdspb.ClientConfig
	for _, c := range all {
		if c.ClientScope == scope {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ClientConfig matched scope=%q", scope)
	}
	return out, nil
}

func xdsConfigCommandRunWithError(cmd *cobra.Command, args []string) error {
	clientStatus := transport.FetchClientStatus()
	if len(clientStatus.Config) == 0 {
		return fmt.Errorf("no ClientConfig returned")
	}
	sortPerXdsConfigs(clientStatus)

	configs, err := filterConfigsByScope(clientStatus.Config, xdsScopeFlag)
	if err != nil {
		return err
	}

	if xdsTypeFlag == "" {
		// Print whole response; if filtered by scope, print a shallow copy to reflect only filtered configs
		if len(configs) == len(clientStatus.Config) {
			return printProtoBufMessageAsJSON(clientStatus)
		}
		return printProtoBufMessageAsJSON(&csdspb.ClientStatusResponse{Config: configs})
	}

	// Parse --type and print resources for each selected config
	wantXdsTypes := strings.Split(xdsTypeFlag, ",")
	var wantLDS, wantRDS, wantCDS, wantEDS bool
	for _, t := range wantXdsTypes {
		switch strings.ToLower(t) {
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

	multi := len(configs) > 1
	for _, cfg := range configs {
		if multi {
			fmt.Printf("== client_scope: %q ==\n", cfg.ClientScope)
		}
		if len(cfg.GenericXdsConfigs) > 0 {
			for _, g := range cfg.GenericXdsConfigs {
				var m proto.Message
				tokens := strings.Split(g.TypeUrl, ".")
				switch tokens[len(tokens)-1] {
				case "Listener":
					if wantLDS {
						m = g.GetXdsConfig()
					}
				case "RouteConfiguration":
					if wantRDS {
						m = g.GetXdsConfig()
					}
				case "Cluster":
					if wantCDS {
						m = g.GetXdsConfig()
					}
				case "ClusterLoadAssignment":
					if wantEDS {
						m = g.GetXdsConfig()
					}
				}
				if m != nil {
					if err := printProtoBufMessageAsJSON(m); err != nil {
						return fmt.Errorf("Failed to print xDS config: %v", err)
					}
				}
			}
		} else {
			for _, x := range cfg.XdsConfig {
				var m proto.Message
				switch x.PerXdsConfig.(type) {
				case *csdspb.PerXdsConfig_ListenerConfig:
					if wantLDS {
						m = x.GetListenerConfig()
					}
				case *csdspb.PerXdsConfig_RouteConfig:
					if wantRDS {
						m = x.GetRouteConfig()
					}
				case *csdspb.PerXdsConfig_ClusterConfig:
					if wantCDS {
						m = x.GetClusterConfig()
					}
				case *csdspb.PerXdsConfig_EndpointConfig:
					if wantEDS {
						m = x.GetEndpointConfig()
					}
				}
				if m != nil {
					if err := printProtoBufMessageAsJSON(m); err != nil {
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
	Scope       string
	Name        string
	Status      adminpb.ClientResourceStatus
	Version     string
	Type        string
	LastUpdated *timestamppb.Timestamp
}

func printStatusEntry(entry *xdsResourceStatusEntry, includeScope bool) {
	if includeScope {
		fmt.Fprintf(
			w, "%v\t%v\t%v\t%v\t%v\t%v\t\n",
			entry.Scope,
			entry.Name,
			entry.Status,
			entry.Version,
			entry.Type,
			prettyTime(entry.LastUpdated),
		)
		return
	}
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
	if len(clientStatus.Config) == 0 {
		return fmt.Errorf("no ClientConfig returned")
	}
	configs, err := filterConfigsByScope(clientStatus.Config, xdsScopeFlag)
	if err != nil {
		return err
	}

	includeScope := xdsScopeFlag == ""
	if includeScope {
		fmt.Fprintln(w, "Scope\tName\tStatus\tVersion\tType\tLastUpdated")
	} else {
		fmt.Fprintln(w, "Name\tStatus\tVersion\tType\tLastUpdated")
	}

	for _, config := range configs {
		scope := config.ClientScope
		for _, g := range config.GenericXdsConfigs {
			entry := xdsResourceStatusEntry{
				Scope:       scope,
				Name:        g.Name,
				Status:      g.ClientStatus,
				Version:     g.VersionInfo,
				Type:        g.TypeUrl,
				LastUpdated: g.LastUpdated,
			}
			printStatusEntry(&entry, includeScope)
		}
		if len(config.GenericXdsConfigs) == 0 {
			for _, x := range config.XdsConfig {
				switch x.PerXdsConfig.(type) {
				case *csdspb.PerXdsConfig_ListenerConfig:
					for _, dl := range x.GetListenerConfig().DynamicListeners {
						e := xdsResourceStatusEntry{Scope: scope, Name: dl.Name, Status: dl.ClientStatus}
						if s := dl.GetActiveState(); s != nil {
							e.Version = s.VersionInfo
							e.Type = s.Listener.TypeUrl
							e.LastUpdated = s.LastUpdated
						}
						printStatusEntry(&e, includeScope)
					}
				case *csdspb.PerXdsConfig_RouteConfig:
					for _, dr := range x.GetRouteConfig().DynamicRouteConfigs {
						e := xdsResourceStatusEntry{
							Scope:       scope,
							Status:      dr.ClientStatus,
							Version:     dr.VersionInfo,
							Type:        dr.RouteConfig.TypeUrl,
							LastUpdated: dr.LastUpdated,
						}
						if packed := dr.GetRouteConfig(); packed != nil {
							var rc routepb.RouteConfiguration
							if err := ptypes.UnmarshalAny(packed, &rc); err != nil {
								return err
							}
							e.Name = rc.Name
						}
						printStatusEntry(&e, includeScope)
					}
				case *csdspb.PerXdsConfig_ClusterConfig:
					for _, dc := range x.GetClusterConfig().DynamicActiveClusters {
						e := xdsResourceStatusEntry{
							Scope:       scope,
							Status:      dc.ClientStatus,
							Version:     dc.VersionInfo,
							Type:        dc.Cluster.TypeUrl,
							LastUpdated: dc.LastUpdated,
						}
						if packed := dc.GetCluster(); packed != nil {
							var c clusterpb.Cluster
							if err := ptypes.UnmarshalAny(packed, &c); err != nil {
								return err
							}
							e.Name = c.Name
						}
						printStatusEntry(&e, includeScope)
					}
				case *csdspb.PerXdsConfig_EndpointConfig:
					for _, de := range x.GetEndpointConfig().GetDynamicEndpointConfigs() {
						e := xdsResourceStatusEntry{
							Scope:       scope,
							Status:      de.ClientStatus,
							Version:     de.VersionInfo,
							Type:        de.EndpointConfig.TypeUrl,
							LastUpdated: de.LastUpdated,
						}
						if packed := de.GetEndpointConfig(); packed != nil {
							var ep endpointpb.ClusterLoadAssignment
							if err := ptypes.UnmarshalAny(packed, &ep); err != nil {
								return err
							}
							e.Name = ep.ClusterName
						}
						printStatusEntry(&e, includeScope)
					}
				}
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
	xdsConfigCmd.Flags().StringVarP(&xdsTypeFlag, "type", "y", "", "Filters the wanted type of xDS config to print (separated by commas) (available types: LDS,RDS,CDS,EDS) (by default, print all)")
	xdsConfigCmd.Flags().StringVarP(&xdsScopeFlag, "scope", "s", "", "Filter by client_scope when multiple ClientConfig are present")
	xdsCmd.AddCommand(xdsConfigCmd)
	xdsCmd.AddCommand(xdsStatusCmd)
	rootCmd.AddCommand(xdsCmd)
}
