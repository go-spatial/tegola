package protocol

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"net"
	"slices"
	"strconv"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// ClientContextOption represents a client context option.
type clientContextOption int8

func (k clientContextOption) valueString(v any) string {
	return fmt.Sprintf("%s: %v", k, v)
}

// ClientContextOption constants.
const (
	ccoVersion            clientContextOption = 1
	ccoType               clientContextOption = 2
	ccoApplicationProgram clientContextOption = 3
)

// ClientContext represents a client context part.
type ClientContext struct {
	options[clientContextOption]
}

// SetVersion sets the client version option.
func (cc *ClientContext) SetVersion(v string) { cc.set(ccoVersion, v) }

// SetType sets the client type option.
func (cc *ClientContext) SetType(v string) { cc.set(ccoType, v) }

// SetApplicationProgram sets the client application program option.
func (cc *ClientContext) SetApplicationProgram(v string) { cc.set(ccoApplicationProgram, v) }

func (cc *ClientContext) decode(dec *encoding.Decoder, header *PartHeader, _ *ReaderAttrs) error {
	return cc.options.decode(dec, header.numArg())
}

// Cdm represents a ConnectOption ClientDistributionMode.
type Cdm byte

// ConnectOption ClientDistributionMode constants.
const (
	CdmOff                 Cdm = 0
	CdmConnection          Cdm = 1
	CdmStatement           Cdm = 2
	CdmConnectionStatement Cdm = 3
)

// dpv represents a ConnectOption DistributionProtocolVersion.
type dpv byte

// distribution protocol version

// ConnectOption DistributionProtocolVersion constants.
const (
	dpvBaseline                       dpv = 0
	dpvClientHandlesStatementSequence dpv = 1
)

// ConnectOption represents a connect option.
type connectOption int8

func (k connectOption) valueString(v any) string {
	return fmt.Sprintf("%s: %v", k, v)
}

// ConnectOption constants.
const (
	coConnectionID                        connectOption = 1
	coCompleteArrayExecution              connectOption = 2  //!< @deprecated Array execution semantics, always true.
	coClientLocale                        connectOption = 3  //!< Client locale information.
	coSupportsLargeBulkOperations         connectOption = 4  //!< Bulk operations >32K are supported.
	coDistributionEnabled                 connectOption = 5  //!< @deprecated Distribution (topology & call routing) enabled
	coPrimaryConnectionID                 connectOption = 6  //!< @deprecated Id of primary connection (unused).
	coPrimaryConnectionHost               connectOption = 7  //!< @deprecated Primary connection host name (unused).
	coPrimaryConnectionPort               connectOption = 8  //!< @deprecated Primary connection port (unused).
	coCompleteDatatypeSupport             connectOption = 9  //!< @deprecated All data types supported (always on).
	coLargeNumberOfParametersSupport      connectOption = 10 //!< Number of parameters >32K is supported.
	coSystemID                            connectOption = 11 //!< SID of SAP HANA Database system (output only).
	coDataFormatVersion                   connectOption = 12 //!< Version of data format used in communication (@see DataFormatVersionEnum).
	coAbapVarcharMode                     connectOption = 13 //!< ABAP varchar mode is enabled (trailing blanks in string constants are trimmed off).
	coSelectForUpdateSupported            connectOption = 14 //!< SELECT FOR UPDATE function code understood by client
	coClientDistributionMode              connectOption = 15 //!< client distribution mode
	coEngineDataFormatVersion             connectOption = 16 //!< Engine version of data format used in communication (@see DataFormatVersionEnum).
	coDistributionProtocolVersion         connectOption = 17 //!< version of distribution protocol handling (@see DistributionProtocolVersionEnum)
	coSplitBatchCommands                  connectOption = 18 //!< permit splitting of batch commands
	coUseTransactionFlagsOnly             connectOption = 19 //!< use transaction flags only for controlling transaction
	coRowSlotImageParameter               connectOption = 20 //!< row-slot image parameter passing
	coIgnoreUnknownParts                  connectOption = 21 //!< server does not abort on unknown parts
	coTableOutputParameterMetadataSupport connectOption = 22 //!< support table type output parameter metadata.
	coDataFormatVersion2                  connectOption = 23 //!< Version of data format used in communication (as DataFormatVersion used wrongly in old servers)
	coItabParameter                       connectOption = 24 //!< bool option to signal abap itab parameter support
	coDescribeTableOutputParameter        connectOption = 25 //!< override "omit table output parameter" setting in this session
	coColumnarResultSet                   connectOption = 26 //!< column wise result passing
	coScrollableResultSet                 connectOption = 27 //!< scrollable result set
	coClientInfoNullValueSupported        connectOption = 28 //!< can handle null values in client info
	coAssociatedConnectionID              connectOption = 29 //!< associated connection id
	coNonTransactionalPrepare             connectOption = 30 //!< can handle and uses non-transactional prepare
	coFdaEnabled                          connectOption = 31 //!< Fast Data Access at all enabled
	coOSUser                              connectOption = 32 //!< client OS user name
	coRowSlotImageResultSet               connectOption = 33 //!< row-slot image result passing
	coEndianness                          connectOption = 34 //!< endianness (@see EndiannessEnumType)
	coUpdateTopologyAnywhere              connectOption = 35 //!< Allow update of topology from any reply
	coEnableArrayType                     connectOption = 36 //!< Enable supporting Array data type
	coImplicitLobStreaming                connectOption = 37 //!< implicit lob streaming
	coCachedViewProperty                  connectOption = 38 //!< provide cached view timestamps to the client
	coXOpenXAProtocolSupported            connectOption = 39 //!< JTA(X/Open XA) Protocol
	coPrimaryCommitRedirectionSupported   connectOption = 40 //!< S2PC routing control
	coActiveActiveProtocolVersion         connectOption = 41 //!< Version of Active/Active protocol
	coActiveActiveConnectionOriginSite    connectOption = 42 //!< Tell where is the anchor connection located. This is unidirectional property from client to server.
	coQueryTimeoutSupported               connectOption = 43 //!< support query timeout (e.g., Statement.setQueryTimeout)
	coFullVersionString                   connectOption = 44 //!< Full version string of the client or server (the sender) (added to hana2sp0)
	coDatabaseName                        connectOption = 45 //!< Database name (string) that we connected to (sent by server) (added to hana2sp0)
	coBuildPlatform                       connectOption = 46 //!< Build platform of the client or server (the sender) (added to hana2sp0)
	coImplicitXASessionSupported          connectOption = 47 //!< S2PC routing control - implicit XA join support after prepare and before execute in MessageType_Prepare, MessageType_Execute and MessageType_PrepareAndExecute
	coClientSideColumnEncryptionVersion   connectOption = 48 //!< Version of client-side column encryption
	coCompressionLevelAndFlags            connectOption = 49 //!< Network compression level and flags (added to hana2sp02)
	coClientSideReExecutionSupported      connectOption = 50 //!< support client-side re-execution for client-side encryption (added to hana2sp03)
	coClientReconnectWaitTimeout          connectOption = 51 //!< client reconnection wait timeout for transparent session recovery
	coOriginalAnchorConnectionID          connectOption = 52 //!< original anchor connectionID to notify client's RECONNECT
	coFlagSet1                            connectOption = 53 //!< flags for aggregating several options
	coTopologyNetworkGroup                connectOption = 54 //!< NetworkGroup name sent by client to choose topology mapping (added to hana2sp04)
	coIPAddress                           connectOption = 55 //!< IP Address of the sender (added to hana2sp04)
	coLRRPingTime                         connectOption = 56 //!< Long running request ping time
	coRedirectionType                     connectOption = 57 //!< Type of HANA Cloud redirection
	coRedirectedHost                      connectOption = 58 //!< Cloud redirected hostname, if redirected
	coRedirectedPort                      connectOption = 59 //!< Cloud redirected port, if redirected
	coEndPointHost                        connectOption = 60 //!< Original hostname from user, before redirection
	coEndPointPort                        connectOption = 61 //!< Original port from user, before redirection
	coEndPointList                        connectOption = 62 //!< Original host:port;host:port list (including scale-out) from user
)

// ConnectOptions represents a connect options part.
type ConnectOptions struct {
	options[connectOption]
}

// DataFormatVersion2OrZero returns the data format version2 option if available, the zero value otherwise.
func (co *ConnectOptions) DataFormatVersion2OrZero() int {
	var v int32
	co.get(coDataFormatVersion2, &v)
	return int(v)
}

// SetDataFormatVersion2 sets the data format version 2 option.
func (co *ConnectOptions) SetDataFormatVersion2(v int) {
	co.set(coDataFormatVersion2, int32(v)) //nolint: gosec
}

// SetClientDistributionMode sets the client distribution mode option.
func (co *ConnectOptions) SetClientDistributionMode(v Cdm) {
	co.set(coClientDistributionMode, int32(v))
}

// ClientDistributionModeOrZero returns the client distribution mode option if available, the zero value otherwise.
func (co *ConnectOptions) ClientDistributionModeOrZero() Cdm {
	var v int32
	co.get(coClientDistributionMode, &v)
	return Cdm(v) //nolint: gosec
}

// SystemIDOrZero returns the system ID option if available, the zero value otherwise.
func (co *ConnectOptions) SystemIDOrZero() string {
	var v string
	co.get(coSystemID, &v)
	return v
}

// SetUpdateTopologyAnywhere sets the update topology anywhere option.
func (co *ConnectOptions) SetUpdateTopologyAnywhere(v bool) {
	co.set(coUpdateTopologyAnywhere, v)
}

// SetSelectForUpdateSupported sets the select for update supported option.
func (co *ConnectOptions) SetSelectForUpdateSupported(v bool) {
	co.set(coSelectForUpdateSupported, v)
}

// Compression flag bits in the int32 value of coCompressionLevelAndFlags.
// See the negotiation doc in session.go for how these are used.
const (
	CoCompressionLZ4Supported int32 = 0x00000100 // sender can decompress LZ4
	CoCompressionLZ4Enabled   int32 = 0x00000200 // sender wants compression on
)

// CompressionLevelAndFlagsOrZero gets the compression level/flags option if available,
// the zero value otherwise.
func (co *ConnectOptions) CompressionLevelAndFlagsOrZero() int32 {
	var v int32
	co.get(coCompressionLevelAndFlags, &v)
	return v
}

// SetCompressionLevelAndFlags sets the compression level/flags option.
func (co *ConnectOptions) SetCompressionLevelAndFlags(v int32) {
	co.set(coCompressionLevelAndFlags, v)
}

// DatabaseNameOrZero returns the database name option if available, the zero value otherwise.
func (co *ConnectOptions) DatabaseNameOrZero() string {
	var v string
	co.get(coDatabaseName, &v)
	return v
}

// ServerConnectionIDOrZero returns the server connection id option if available, the zero value otherwise.
// The server connection id identifies the session on the server and is used in the session management statements
// (alter system cancel|disconnect session). The session id of the message header cannot be used for this.
func (co *ConnectOptions) ServerConnectionIDOrZero() int {
	var v int32
	co.get(coConnectionID, &v)
	return int(v)
}

// FullVersionOrZero returns the full version option if available, the zero value otherwise.
func (co *ConnectOptions) FullVersionOrZero() string {
	var v string
	co.get(coFullVersionString, &v)
	return v
}

// SetClientLocale sets the client locale option.
func (co *ConnectOptions) SetClientLocale(v string) { co.set(coClientLocale, v) }

func (co *ConnectOptions) decode(dec *encoding.Decoder, header *PartHeader, _ *ReaderAttrs) error {
	return co.options.decode(dec, header.numArg())
}

// DBConnectInfoType represents a database connect info type.
type dbConnectInfoType int8

func (k dbConnectInfoType) valueString(v any) string {
	return fmt.Sprintf("%s: %v", k, v)
}

// DBConnectInfoType constants.
const (
	ciDatabaseName dbConnectInfoType = 1 // string
	ciHost         dbConnectInfoType = 2 // string
	ciPort         dbConnectInfoType = 3 // int4
	ciIsConnected  dbConnectInfoType = 4 // bool
)

// DBConnectInfo represents a database connect info part.
type DBConnectInfo struct {
	options[dbConnectInfoType]
}

// SetDatabaseName sets the database name option.
func (ci *DBConnectInfo) SetDatabaseName(v string) { ci.set(ciDatabaseName, v) }

// HostOrZero returns the host option, the zero value otherwise.
func (ci *DBConnectInfo) HostOrZero() string { var v string; ci.get(ciHost, &v); return v }

// PortOrZero returns the port option, the zero value otherwise.
func (ci *DBConnectInfo) PortOrZero() int { var v int32; ci.get(ciPort, &v); return int(v) }

// IsConnectedOrZero returns this IsConnected option, the zero value otherwise.
func (ci *DBConnectInfo) IsConnectedOrZero() bool {
	var v bool
	ci.get(ciIsConnected, &v)
	return v
}

func (ci *DBConnectInfo) decode(dec *encoding.Decoder, header *PartHeader, _ *ReaderAttrs) error {
	return ci.options.decode(dec, header.numArg())
}

type statementContextType int8

func (k statementContextType) valueString(v any) string {
	return fmt.Sprintf("%s: %v", k, v)
}

const (
	scStatementSequenceInfo         statementContextType = 1
	scServerProcessingTime          statementContextType = 2
	scSchemaName                    statementContextType = 3
	scFlagSet                       statementContextType = 4
	scQueryTimeout                  statementContextType = 5
	scClientReconnectionWaitTimeout statementContextType = 6
	scServerCPUTime                 statementContextType = 7
	scServerMemoryUsage             statementContextType = 8
)

type statementContext struct {
	options[statementContextType]
}

func (sc *statementContext) decode(dec *encoding.Decoder, header *PartHeader, _ *ReaderAttrs) error {
	return sc.options.decode(dec, header.numArg())
}

// transaction flags.
type transactionFlagType int8

func (k transactionFlagType) valueString(v any) string {
	return fmt.Sprintf("%s: %v", k, v)
}

const (
	tfRolledback                     transactionFlagType = 0
	tfCommitted                      transactionFlagType = 1
	tfNewIsolationLevel              transactionFlagType = 2
	tfDDLCommitmodeChanged           transactionFlagType = 3
	tfWriteTransactionStarted        transactionFlagType = 4
	tfNowriteTransactionStarted      transactionFlagType = 5
	tfSessionClosingTransactionError transactionFlagType = 6
	tfReadOnlyMode                   transactionFlagType = 7
	tfLast                           transactionFlagType = 8
)

type transactionFlags struct {
	options[transactionFlagType]
}

func (tf *transactionFlags) decode(dec *encoding.Decoder, header *PartHeader, _ *ReaderAttrs) error {
	return tf.options.decode(dec, header.numArg())
}

type topologyOption int8

func (k topologyOption) valueString(v any) string {
	switch k {
	case toServiceType:
		v := v.(int32)
		return fmt.Sprintf("%s: %v", k, ServiceType(v))
	default:
		return fmt.Sprintf("%s: %v", k, v)
	}
}

const (
	toHostName         topologyOption = 1
	toHostPortnumber   topologyOption = 2
	toTenantName       topologyOption = 3
	toLoadfactor       topologyOption = 4
	toVolumeID         topologyOption = 5
	toIsPrimary        topologyOption = 6
	toIsCurrentSession topologyOption = 7
	toServiceType      topologyOption = 8
	toNetworkDomain    topologyOption = 9 // deprecated
	toIsStandby        topologyOption = 10
	toAllIPAddresses   topologyOption = 11 // deprecated
	toAllHostNames     topologyOption = 12 // deprecated
	toSiteType         topologyOption = 13
)

// ServiceType represents a service type.
type ServiceType int32

// Service type constants.
const (
	StOther            ServiceType = 0
	StNameServer       ServiceType = 1
	StPreprocessor     ServiceType = 2
	StIndexServer      ServiceType = 3
	StStatisticsServer ServiceType = 4
	StXSEngine         ServiceType = 5
	StReserved6        ServiceType = 6
	StCompileServer    ServiceType = 7
	StDPServer         ServiceType = 8
	StDIServer         ServiceType = 9
	StComputeServer    ServiceType = 10
	StScriptServer     ServiceType = 11
)

// SiteType represents the HSR (HANA System Replication) site type of a topology
// host. Mirrors the reference client's SiteType enum (Layout.hpp).
type SiteType int32

// Site type constants.
const (
	SiteTypeNone      SiteType = 0 // no HSR
	SiteTypePrimary   SiteType = 1
	SiteTypeSecondary SiteType = 2
	SiteTypeTertiary  SiteType = 3
)

// SiteVolumeID is the packed site + volume identifier the server reports for a
// topology host (topologyOption toVolumeID): site ID in the high 8 bits, volume
// ID in the low 24 bits. Mirrors the reference client's SiteVolumeID.
type SiteVolumeID uint32

const (
	volumeIDMask = 0x00FFFFFF // low 24 bits
	siteIDMask   = 0xFF000000 // high 8 bits
)

// SiteID returns the site id (high 8 bits).
func (id SiteVolumeID) SiteID() uint8 { return uint8(id >> 24) }

// VolumeID returns the volume id (low 24 bits).
func (id SiteVolumeID) VolumeID() uint32 { return uint32(id) & volumeIDMask }

// IsInvalid reports whether either the site id or the volume id is the reserved
// all-ones invalid value (site 0xFF or volume 0xFFFFFF). The server must never
// send such a record; the reference client rejects the whole topology if it
// does.
func (id SiteVolumeID) IsInvalid() bool {
	return uint32(id)&volumeIDMask == volumeIDMask || uint32(id)&siteIDMask == siteIDMask
}

// RoutingNode holds the topology data of a routing node.
type RoutingNode struct {
	SiteVolumeID SiteVolumeID
	SiteType     SiteType
	// Host is the dial address (host+port). It is the reachability key: routing
	// marks every topology entry with the same host reachable or unreachable.
	// It is not the state identity: routing state is preserved by site volume
	// identity (SiteVolumeID, SiteType), so the server must not list the same
	// host with conflicting routing attributes.
	Host    string
	Standby bool // node is a standby node
}

func (n *RoutingNode) sortKey() uint64 {
	return uint64(n.SiteVolumeID)<<32 | uint64(n.SiteType) //nolint:gosec // SiteType enum values 0-3 fit in uint64
}

// Compare compares two routing nodes by their state identity (SiteVolumeID,
// then SiteType), matching the sort order of SortedNodeList.
func (n *RoutingNode) Compare(o *RoutingNode) int {
	return cmp.Compare(n.sortKey(), o.sortKey())
}

// RoutingNodeList is a topology node list.
type RoutingNodeList []*RoutingNode

// Equal reports whether two topologies are equal: the same nodes in the same
// order with equal routing attributes.
func (n RoutingNodeList) Equal(o RoutingNodeList) bool {
	if len(n) != len(o) {
		return false
	}
	for i := range n {
		if *n[i] != *o[i] {
			return false
		}
	}
	return true
}

// RoutingNodes defines the interface to fetch routing nodes.
type RoutingNodes interface {
	SortedNodeList(host *string) (RoutingNodeList, error)
}

var _ RoutingNodes = (*TopologyInformation)(nil)

// TopologyInformation represents a topology information part.
type TopologyInformation struct {
	nodes []*options[topologyOption]
}

func (ti TopologyInformation) String() string { return fmt.Sprintf("%v", ti.nodes) }

func (ti *TopologyInformation) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numArg := header.numArg()

	ti.nodes = slices.Grow(ti.nodes, numArg)[:numArg]
	for i := range numArg {
		node := &options[topologyOption]{}
		ti.nodes[i] = node
		hostNumArg := int(dec.Int16())
		if err := node.decode(dec, hostNumArg); err != nil {
			return err
		}
	}
	return nil
}

var errInvalidSiteVolumeID = errors.New("invalid site volume id")
var errPortForwarded = errors.New("port forwarded")
var errBadTopologyNoOwnRecord = errors.New("bad topology error - no own record")
var errBadTopologyDuplicateKey = errors.New("bad topology error - duplicate key")

// SortedNodeList implements the RoutingNodes interface.
func (ti TopologyInformation) SortedNodeList(routingHost *string) (RoutingNodeList, error) {
	rnl := make(RoutingNodeList, 0, len(ti.nodes))

	currentSessionFound := false

	for _, node := range ti.nodes {
		var serviceType int32
		node.get(toServiceType, &serviceType)
		if ServiceType(serviceType) != StIndexServer {
			continue
		}
		var isCurrentSession bool
		node.get(toIsCurrentSession, &isCurrentSession)
		if isCurrentSession {
			currentSessionFound = true
		}

		var host string
		node.get(toHostName, &host)
		var i32 int32
		node.get(toHostPortnumber, &i32)
		port := strconv.Itoa(int(i32))

		// if host provided check port forwarding
		if isCurrentSession && routingHost != nil {
			// check if port forwarded (connection is behind a port-remapping proxy)
			// in which case the routing node addresses are unreachable and routing must be
			// disabled.
			// It checks the server's own record (IsCurrentSession) and compares the port the
			// server reports for itself against the port we actually connected on. Host is
			// deliberately NOT compared: the routing host is the server's internal name and
			// legitimately differs from the dialed address on healthy direct connections. A
			// port mismatch is the signature of an HA-proxy / port-forward remapping the
			// listener port.
			// This is an intentionally incomplete heuristic, matching the C++ reference.
			// It only detects proxies that REMAP THE PORT. A NAT/proxy that rewrites the host but
			// preserves the port (client port == internal port) is a false negative.
			_, routingPort, err := net.SplitHostPort(*routingHost)
			if err != nil {
				return nil, err
			}
			if port != routingPort {
				return nil, errPortForwarded
			}
		}

		rn := new(RoutingNode)

		rn.Host = net.JoinHostPort(host, port)

		var volumeID int32 // sign
		node.get(toVolumeID, &volumeID)
		rn.SiteVolumeID = SiteVolumeID(volumeID) //nolint: gosec // unsigned - see C++)

		var isStandby bool
		node.get(toIsStandby, &isStandby)
		rn.Standby = isStandby

		var siteType int32
		node.get(toSiteType, &siteType)
		rn.SiteType = SiteType(siteType)

		if rn.SiteVolumeID.IsInvalid() {
			return nil, errInvalidSiteVolumeID
		}

		// build sorted routingNodes
		n, found := slices.BinarySearchFunc(rnl, rn, func(a, b *RoutingNode) int {
			return cmp.Compare(a.sortKey(), b.sortKey())
		})
		if found {
			return nil, errBadTopologyDuplicateKey
		}
		rnl = slices.Insert(rnl, n, rn)
	}
	if !currentSessionFound {
		return nil, errBadTopologyNoOwnRecord
	}
	return rnl, nil
}

type optionsType interface {
	~int8
	valueString(v any) string
}

// options represents a generic option part.
type options[K optionsType] map[K]any

func (ops options[K]) String() string {
	s := make([]string, 0, len(ops))
	for k, v := range ops {
		if b, ok := v.([]byte); ok {
			s = append(s, fmt.Sprintf("%v: %x", k, b))
		} else {
			s = append(s, k.valueString(v))
		}
	}
	slices.Sort(s)
	return fmt.Sprintf("%v", s)
}

func (ops *options[K]) get(k K, v any) bool {
	if *ops == nil {
		return false
	}
	mv, ok := (*ops)[k]
	if !ok {
		return false
	}
	switch v := v.(type) {
	case *string:
		*v = mv.(string)
	case *bool:
		*v = mv.(bool)
	case *int32:
		*v = mv.(int32)
	case *float64:
		*v = mv.(float64)
	default:
		panic("invalid option type")
	}
	return true
}

func (ops *options[K]) set(k K, v any) {
	if *ops == nil {
		*ops = options[K]{}
	}
	(*ops)[k] = v
}

func (ops options[K]) numArg() int { return len(ops) }

func (ops *options[K]) decode(dec *encoding.Decoder, numArg int) error {
	*ops = options[K]{} // no reuse of maps - create new one
	for range numArg {
		k := K(dec.Int8())

		switch typeCode(dec.Byte()) {
		case tcBoolean:
			(*ops)[k] = dec.Bool()
		case tcTinyint:
			(*ops)[k] = dec.Int8()
		case tcInteger:
			(*ops)[k] = dec.Int32()
		case tcBigint:
			(*ops)[k] = dec.Int64()
		case tcDouble:
			(*ops)[k] = dec.Float64()
		case tcString:
			size := int(dec.Int16())
			(*ops)[k] = dec.Str(size)
		case tcBstring:
			size := int(dec.Int16())
			(*ops)[k] = dec.Bytes(size)
		default:
			panic("unknown option typeCode") // should never happen
		}
	}
	return nil
}

func (ops options[K]) encode(enc *encoding.Encoder) error {
	for k, v := range ops {
		enc.Int8(int8(k))

		switch v := v.(type) {
		case bool:
			enc.Byte(byte(tcBoolean))
			enc.Bool(v)
		case int8:
			enc.Byte(byte(tcTinyint))
			enc.Int8(v)
		case int32:
			enc.Byte(byte(tcInteger))
			enc.Int32(v)
		case int64:
			enc.Byte(byte(tcBigint))
			enc.Int64(v)
		case float64:
			enc.Byte(byte(tcDouble))
			enc.Float64(v)
		case string:
			if len(v) > math.MaxInt16 {
				return fmt.Errorf("option value length %d exceeds protocol maximum %d", len(v), math.MaxInt16)
			}
			enc.Byte(byte(tcString))
			enc.Int16(int16(len(v))) //nolint: gosec // bounded by the guard above
			enc.Bytes([]byte(v))
		case []byte:
			if len(v) > math.MaxInt16 {
				return fmt.Errorf("option value length %d exceeds protocol maximum %d", len(v), math.MaxInt16)
			}
			enc.Byte(byte(tcBstring))
			enc.Int16(int16(len(v))) //nolint: gosec // bounded by the guard above
			enc.Bytes(v)
		default:
			panic("option type not implemented") // should never happen
		}
	}
	return nil
}
