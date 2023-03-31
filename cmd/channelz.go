package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/golang/protobuf/ptypes"
	timestamppb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"
	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"
	"github.com/spf13/cobra"
	zpb "google.golang.org/grpc/channelz/grpc_channelz_v1"
)

var (
	jsonOutputFlag bool
	startIDFlag    int64
	maxResultsFlag int64
)

func prettyTime(ts *timestamppb.Timestamp) string {
	if ts == nil || (ts.Seconds == 0 && ts.Nanos == 0) {
		return ""
	}
	if timestampFlag {
		return ptypes.TimestampString(ts)
	}
	t, _ := ptypes.Timestamp(ts)
	return humanize.Time(t)
}

func prettyAddress(addr *zpb.Address) string {
	if ipPort := addr.GetTcpipAddress(); ipPort != nil {
		address := net.TCPAddr{IP: net.IP(ipPort.IpAddress), Port: int(ipPort.Port)}
		return address.String()
	}
	panic(fmt.Sprintf("Address type not supported for %s", addr))
}

func printChannelTraceEvents(events []*zpb.ChannelTraceEvent) {
	fmt.Fprintln(w, "Severity\tTime\tChild Ref\tDescription\t")
	for _, event := range events {
		var childRef string
		switch event.ChildRef.(type) {
		case *zpb.ChannelTraceEvent_SubchannelRef:
			childRef = fmt.Sprintf("subchannel(%v)", event.GetSubchannelRef())
		case *zpb.ChannelTraceEvent_ChannelRef:
			childRef = fmt.Sprintf("channel(%v)", event.GetChannelRef())
		}
		fmt.Fprintf(
			w, "%v\t%v\t%v\t%v\t\n",
			event.Severity,
			prettyTime(event.Timestamp),
			childRef,
			event.Description,
		)
	}
	w.Flush()
}

func printSockets(sockets []*zpb.Socket) {
	fmt.Fprintln(w, "Socket ID\tLocal->Remote\tStreams(Started/Succeeded/Failed)\tMessages(Sent/Received)\t")
	for _, socket := range sockets {
		if socket.GetRef() == nil || socket.GetData() == nil {
			verbose.Debugf("failed to print socket: %s", socket)
			continue
		}
		fmt.Fprintf(
			w, "%v\t%v\t%v/%v/%v\t%v/%v\t\n",
			socket.Ref.SocketId,
			fmt.Sprintf("%v->%v", prettyAddress(socket.Local), prettyAddress(socket.Remote)),
			socket.Data.StreamsStarted,
			socket.Data.StreamsSucceeded,
			socket.Data.StreamsFailed,
			socket.Data.MessagesSent,
			socket.Data.MessagesReceived,
		)
	}
	w.Flush()
}

func printObjectAsJSON(data interface{}) error {
	json, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(json))
	return nil
}

func printCreationTimestamp(data *zpb.ChannelData) string {
	return prettyTime(data.GetTrace().GetCreationTimestamp())
}

func channelzChannelsCommandRunWithError(cmd *cobra.Command, args []string) error {
	var channels = transport.Channels(startIDFlag, maxResultsFlag)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(channels)
	}
	// Print as table
	fmt.Fprintln(w, "Channel ID\tTarget\tState\tCalls(Started/Succeeded/Failed)\tCreated Time\t")
	for _, channel := range channels {
		if channel.GetRef() == nil || channel.GetData() == nil {
			verbose.Debugf("failed to print channel: %s", channel)
			continue
		}
		fmt.Fprintf(
			w, "%v\t%v\t%v\t%v/%v/%v\t%v\t\n",
			channel.Ref.ChannelId,
			channel.Data.Target,
			channel.Data.GetState().GetState(),
			channel.Data.CallsStarted,
			channel.Data.CallsSucceeded,
			channel.Data.CallsFailed,
			printCreationTimestamp(channel.Data),
		)
	}
	w.Flush()
	return nil
}

var channelzChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List client channels for the target application.",
	Args:  cobra.NoArgs,
	RunE:  channelzChannelsCommandRunWithError,
}

func channelzChannelCommandRunWithError(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse ID=%v: %v", args[0], err)
	}
	selected := transport.Channel(id)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(selected)
	}
	// Print as table
	// Print Channel information
	fmt.Fprintf(w, "Channel ID:\t%v\t\n", selected.GetRef().GetChannelId())
	fmt.Fprintf(w, "Target:\t%v\t\n", selected.GetData().GetTarget())
	fmt.Fprintf(w, "State:\t%v\t\n", selected.GetData().GetState().GetState())
	fmt.Fprintf(w, "Calls Started:\t%v\t\n", selected.GetData().GetCallsStarted())
	fmt.Fprintf(w, "Calls Succeeded:\t%v\t\n", selected.GetData().GetCallsSucceeded())
	fmt.Fprintf(w, "Calls Failed:\t%v\t\n", selected.GetData().GetCallsFailed())
	fmt.Fprintf(w, "Created Time:\t%v\t\n", printCreationTimestamp(selected.GetData()))
	w.Flush()
	// Print Subchannel list
	if len(selected.GetSubchannelRef()) > 0 {
		fmt.Println("---")
		fmt.Fprintln(w, "Subchannel ID\tTarget\tState\tCalls(Started/Succeeded/Failed)\tCreatedTime\t")
		for _, subchannelRef := range selected.GetSubchannelRef() {
			var subchannel = transport.Subchannel(subchannelRef.GetSubchannelId())
			if subchannel.GetRef() == nil || subchannel.GetData() == nil {
				verbose.Debugf("failed to print subchannel: %s", subchannel)
				continue
			}
			fmt.Fprintf(
				w, "%v\t%.50s\t%v\t%v/%v/%v\t%v\t\n",
				subchannel.Ref.SubchannelId,
				subchannel.Data.Target,
				subchannel.Data.State.State,
				subchannel.Data.CallsStarted,
				subchannel.Data.CallsSucceeded,
				subchannel.Data.CallsFailed,
				printCreationTimestamp(subchannel.Data),
			)
		}
		w.Flush()
	}
	// Print channel trace events
	if len(selected.GetData().GetTrace().GetEvents()) != 0 {
		fmt.Println("---")
		printChannelTraceEvents(selected.Data.Trace.Events)
	}
	return nil
}

var channelzChannelCmd = &cobra.Command{
	Use:   "channel <channel id or URL>",
	Short: "Display channel states in a human readable way.",
	Args:  cobra.ExactArgs(1),
	RunE:  channelzChannelCommandRunWithError,
}

func channelzSubchannelCommandRunWithError(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse ID=%v: %v", args[0], err)
	}
	selected := transport.Subchannel(id)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(selected)
	}
	// Print as table
	// Print Subchannel information
	fmt.Fprintf(w, "Subchannel ID:\t%v\t\n", selected.GetRef().GetSubchannelId())
	fmt.Fprintf(w, "Target:\t%v\t\n", selected.GetData().GetTarget())
	fmt.Fprintf(w, "State:\t%v\t\n", selected.GetData().GetState().GetState())
	fmt.Fprintf(w, "Calls Started:\t%v\t\n", selected.GetData().GetCallsStarted())
	fmt.Fprintf(w, "Calls Succeeded:\t%v\t\n", selected.GetData().GetCallsSucceeded())
	fmt.Fprintf(w, "Calls Failed:\t%v\t\n", selected.GetData().GetCallsFailed())
	fmt.Fprintf(w, "Created Time:\t%v\t\n", printCreationTimestamp(selected.GetData()))
	w.Flush()
	if len(selected.SocketRef) > 0 {
		// Print socket list
		fmt.Println("---")
		var sockets []*zpb.Socket
		for _, socketRef := range selected.GetSocketRef() {
			sockets = append(sockets, transport.Socket(socketRef.GetSocketId()))
		}
		printSockets(sockets)
	}
	return nil
}

var channelzSubchannelCmd = &cobra.Command{
	Use:   "subchannel <id>",
	Short: "Display subchannel states in a human readable way.",
	Args:  cobra.ExactArgs(1),
	RunE:  channelzSubchannelCommandRunWithError,
}

func channelzSocketCommandRunWithError(cmd *cobra.Command, args []string) error {
	socketID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid socket ID %v", socketID)
	}
	selected := transport.Socket(socketID)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(selected)
	}
	// Print as table
	// Print Socket information
	fmt.Fprintf(w, "Socket ID:\t%v\t\n", selected.GetRef().GetSocketId())
	fmt.Fprintf(w, "Address:\t%v\t\n", fmt.Sprintf("%v->%v", prettyAddress(selected.GetLocal()), prettyAddress(selected.GetRemote())))
	fmt.Fprintf(w, "Streams Started:\t%v\t\n", selected.GetData().GetStreamsStarted())
	fmt.Fprintf(w, "Streams Succeeded:\t%v\t\n", selected.GetData().GetStreamsSucceeded())
	fmt.Fprintf(w, "Streams Failed:\t%v\t\n", selected.GetData().GetStreamsFailed())
	fmt.Fprintf(w, "Messages Sent:\t%v\t\n", selected.GetData().GetMessagesSent())
	fmt.Fprintf(w, "Messages Received:\t%v\t\n", selected.GetData().GetMessagesReceived())
	fmt.Fprintf(w, "Keep Alives Sent:\t%v\t\n", selected.GetData().GetKeepAlivesSent())
	fmt.Fprintf(w, "Last Local Stream Created:\t%v\t\n", prettyTime(selected.GetData().GetLastLocalStreamCreatedTimestamp()))
	fmt.Fprintf(w, "Last Remote Stream Created:\t%v\t\n", prettyTime(selected.GetData().GetLastRemoteStreamCreatedTimestamp()))
	fmt.Fprintf(w, "Last Message Sent Created:\t%v\t\n", prettyTime(selected.GetData().GetLastMessageSentTimestamp()))
	fmt.Fprintf(w, "Last Message Received Created:\t%v\t\n", prettyTime(selected.GetData().GetLastMessageReceivedTimestamp()))
	fmt.Fprintf(w, "Local Flow Control Window:\t%v\t\n", selected.GetData().GetLocalFlowControlWindow().GetValue())
	fmt.Fprintf(w, "Remote Flow Control Window:\t%v\t\n", selected.GetData().GetRemoteFlowControlWindow().GetValue())
	w.Flush()
	if len(selected.GetData().GetOption()) > 0 {
		fmt.Println("---")
		fmt.Fprintln(w, "Socket Options Name\tValue\t")
		for _, option := range selected.GetData().GetOption() {
			if option.GetValue() != "" {
				// Prefer human readable value than the Any proto
				fmt.Fprintf(w, "%v\t%v\t\n", option.GetName(), option.GetValue())
			} else {
				fmt.Fprintf(w, "%v\t%v\t\n", option.GetName(), option.GetAdditional())
			}
		}
		w.Flush()
	}
	// Print security information
	if security := selected.GetSecurity(); security != nil {
		fmt.Println("---")
		switch x := security.Model.(type) {
		case *zpb.Security_Tls_:
			fmt.Fprintf(w, "Security Model:\t%v\t\n", "TLS")
			switch y := security.GetTls().GetCipherSuite().(type) {
			case *zpb.Security_Tls_StandardName:
				fmt.Fprintf(w, "Standard Name:\t%v\t\n", security.GetTls().GetStandardName())
			case *zpb.Security_Tls_OtherName:
				fmt.Fprintf(w, "Other Name:\t%v\t\n", security.GetTls().GetOtherName())
			default:
				return fmt.Errorf("Unexpected Cipher suite name type %T", y)
			}
			// fmt.Fprintf(w, "Local Certificate:\t%v\t\n", security.GetTls().LocalCertificate)
			// fmt.Fprintf(w, "Remote Certificate:\t%v\t\n", security.GetTls().RemoteCertificate)
		case *zpb.Security_Other:
			fmt.Fprintf(w, "Security Model:\t%v\t\n", "Other")
			fmt.Fprintf(w, "Name:\t%v\t\n", security.GetOther().GetName())
			// fmt.Fprintf(w, "Value:\t%v\t\n", security.GetOther().Value)
		default:
			return fmt.Errorf("Unexpected security model type %T", x)
		}
		w.Flush()
	}
	return nil
}

var channelzSocketCmd = &cobra.Command{
	Use:   "socket <id>",
	Short: "Display socket states in a human readable way.",
	Args:  cobra.ExactArgs(1),
	RunE:  channelzSocketCommandRunWithError,
}

func channelzServersCommandRunWithError(cmd *cobra.Command, args []string) error {
	var servers = transport.Servers(startIDFlag, maxResultsFlag)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(servers)
	}
	// Print as table
	fmt.Fprintln(w, "Server ID\tListen Addresses\tCalls(Started/Succeeded/Failed)\tLast Call Started\t")
	for _, server := range servers {
		var listenAddresses []string
		for _, socketRef := range server.GetListenSocket() {
			socket := transport.Socket(socketRef.SocketId)
			listenAddresses = append(listenAddresses, prettyAddress(socket.GetLocal()))
		}
		fmt.Fprintf(
			w, "%v\t%v\t%v/%v/%v\t%v\t\n",
			server.GetRef().GetServerId(),
			listenAddresses,
			server.GetData().GetCallsStarted(),
			server.GetData().GetCallsSucceeded(),
			server.GetData().GetCallsFailed(),
			prettyTime(server.GetData().GetLastCallStartedTimestamp()),
		)
	}
	w.Flush()
	return nil
}

var channelzServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "List servers in a human readable way.",
	Args:  cobra.NoArgs,
	RunE:  channelzServersCommandRunWithError,
}

func channelzServerCommandRunWithError(cmd *cobra.Command, args []string) error {
	serverID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid server ID %v", serverID)
	}
	selected := transport.Server(serverID)
	// Print as JSON
	if jsonOutputFlag {
		return printObjectAsJSON(selected)
	}
	// Print as table
	var listenAddresses []string
	for _, socketRef := range selected.GetListenSocket() {
		socket := transport.Socket(socketRef.GetSocketId())
		listenAddresses = append(listenAddresses, prettyAddress(socket.GetLocal()))
	}
	fmt.Fprintf(w, "Server Id:\t%v\t\n", selected.GetRef().GetServerId())
	fmt.Fprintf(w, "Listen Addresses:\t%v\t\n", listenAddresses)
	fmt.Fprintf(w, "Calls Started:\t%v\t\n", selected.GetData().GetCallsStarted())
	fmt.Fprintf(w, "Calls Succeeded:\t%v\t\n", selected.GetData().GetCallsSucceeded())
	fmt.Fprintf(w, "Calls Failed:\t%v\t\n", selected.GetData().GetCallsFailed())
	fmt.Fprintf(w, "Last Call Started:\t%v\t\n", prettyTime(selected.GetData().GetLastCallStartedTimestamp()))
	w.Flush()
	if sockets := transport.ServerSocket(selected.GetRef().GetServerId(), startIDFlag, maxResultsFlag); len(sockets) > 0 {
		// Print socket list
		fmt.Println("---")
		printSockets(sockets)
	}
	return nil
}

var channelzServerCmd = &cobra.Command{
	Use:   "server <id>",
	Short: "Display the server state in a human readable way.",
	Args:  cobra.ExactArgs(1),
	RunE:  channelzServerCommandRunWithError,
}

var channelzCmd = &cobra.Command{
	Use:   "channelz",
	Short: "Display gRPC states in a human readable way.",
	Args:  cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(channelzCmd)
	channelzChannelsCmd.Flags().Int64VarP(&maxResultsFlag, "max_results", "m", 100, "The maximum number of output channels")
	channelzChannelsCmd.Flags().Int64VarP(&startIDFlag, "start_id", "s", 0, "The start channel ID")
	channelzServerCmd.Flags().Int64VarP(&maxResultsFlag, "max_results", "m", 100, "The maximum number of the output sockets")
	channelzServerCmd.Flags().Int64VarP(&startIDFlag, "start_id", "s", 0, "The start server socket ID")
	channelzServersCmd.Flags().Int64VarP(&maxResultsFlag, "max_results", "m", 100, "The maximum number of output servers")
	channelzServersCmd.Flags().Int64VarP(&startIDFlag, "start_id", "s", 0, "The start server ID")
	channelzCmd.PersistentFlags().BoolVarP(&jsonOutputFlag, "json", "o", false, "Whether to print the result as JSON")
	channelzCmd.AddCommand(channelzChannelCmd)
	channelzCmd.AddCommand(channelzChannelsCmd)
	channelzCmd.AddCommand(channelzSubchannelCmd)
	channelzCmd.AddCommand(channelzSocketCmd)
	channelzCmd.AddCommand(channelzServersCmd)
	channelzCmd.AddCommand(channelzServerCmd)
}
