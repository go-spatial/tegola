package protocol

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
type ConnectOption int8

// ConnectOption constants.
const (
	coConnectionID                        ConnectOption = 1
	CoCompleteArrayExecution              ConnectOption = 2  //!< @deprecated Array execution semantics, always true.
	CoClientLocale                        ConnectOption = 3  //!< Client locale information.
	coSupportsLargeBulkOperations         ConnectOption = 4  //!< Bulk operations >32K are supported.
	coDistributionEnabled                 ConnectOption = 5  //!< @deprecated Distribution (topology & call routing) enabled
	coPrimaryConnectionID                 ConnectOption = 6  //!< @deprecated Id of primary connection (unused).
	coPrimaryConnectionHost               ConnectOption = 7  //!< @deprecated Primary connection host name (unused).
	coPrimaryConnectionPort               ConnectOption = 8  //!< @deprecated Primary connection port (unused).
	coCompleteDatatypeSupport             ConnectOption = 9  //!< @deprecated All data types supported (always on).
	coLargeNumberOfParametersSupport      ConnectOption = 10 //!< Number of parameters >32K is supported.
	coSystemID                            ConnectOption = 11 //!< SID of SAP HANA Database system (output only).
	coDataFormatVersion                   ConnectOption = 12 //!< Version of data format used in communication (@see DataFormatVersionEnum).
	coAbapVarcharMode                     ConnectOption = 13 //!< ABAP varchar mode is enabled (trailing blanks in string constants are trimmed off).
	CoSelectForUpdateSupported            ConnectOption = 14 //!< SELECT FOR UPDATE function code understood by client
	CoClientDistributionMode              ConnectOption = 15 //!< client distribution mode
	coEngineDataFormatVersion             ConnectOption = 16 //!< Engine version of data format used in communication (@see DataFormatVersionEnum).
	CoDistributionProtocolVersion         ConnectOption = 17 //!< version of distribution protocol handling (@see DistributionProtocolVersionEnum)
	CoSplitBatchCommands                  ConnectOption = 18 //!< permit splitting of batch commands
	coUseTransactionFlagsOnly             ConnectOption = 19 //!< use transaction flags only for controlling transaction
	coRowSlotImageParameter               ConnectOption = 20 //!< row-slot image parameter passing
	coIgnoreUnknownParts                  ConnectOption = 21 //!< server does not abort on unknown parts
	coTableOutputParameterMetadataSupport ConnectOption = 22 //!< support table type output parameter metadata.
	CoDataFormatVersion2                  ConnectOption = 23 //!< Version of data format used in communication (as DataFormatVersion used wrongly in old servers)
	coItabParameter                       ConnectOption = 24 //!< bool option to signal abap itab parameter support
	coDescribeTableOutputParameter        ConnectOption = 25 //!< override "omit table output parameter" setting in this session
	coColumnarResultSet                   ConnectOption = 26 //!< column wise result passing
	coScrollableResultSet                 ConnectOption = 27 //!< scrollable result set
	coClientInfoNullValueSupported        ConnectOption = 28 //!< can handle null values in client info
	coAssociatedConnectionID              ConnectOption = 29 //!< associated connection id
	coNonTransactionalPrepare             ConnectOption = 30 //!< can handle and uses non-transactional prepare
	coFdaEnabled                          ConnectOption = 31 //!< Fast Data Access at all enabled
	coOSUser                              ConnectOption = 32 //!< client OS user name
	coRowSlotImageResultSet               ConnectOption = 33 //!< row-slot image result passing
	coEndianness                          ConnectOption = 34 //!< endianness (@see EndiannessEnumType)
	coUpdateTopologyAnwhere               ConnectOption = 35 //!< Allow update of topology from any reply
	coEnableArrayType                     ConnectOption = 36 //!< Enable supporting Array data type
	coImplicitLobStreaming                ConnectOption = 37 //!< implicit lob streaming
	coCachedViewProperty                  ConnectOption = 38 //!< provide cached view timestamps to the client
	coXOpenXAProtocolSupported            ConnectOption = 39 //!< JTA(X/Open XA) Protocol
	coPrimaryCommitRedirectionSupported   ConnectOption = 40 //!< S2PC routing control
	coActiveActiveProtocolVersion         ConnectOption = 41 //!< Version of Active/Active protocol
	coActiveActiveConnectionOriginSite    ConnectOption = 42 //!< Tell where is the anchor connection located. This is unidirectional property from client to server.
	coQueryTimeoutSupported               ConnectOption = 43 //!< support query timeout (e.g., Statement.setQueryTimeout)
	CoFullVersionString                   ConnectOption = 44 //!< Full version string of the client or server (the sender) (added to hana2sp0)
	CoDatabaseName                        ConnectOption = 45 //!< Database name (string) that we connected to (sent by server) (added to hana2sp0)
	coBuildPlatform                       ConnectOption = 46 //!< Build platform of the client or server (the sender) (added to hana2sp0)
	coImplicitXASessionSupported          ConnectOption = 47 //!< S2PC routing control - implicit XA join support after prepare and before execute in MessageType_Prepare, MessageType_Execute and MessageType_PrepareAndExecute
	coClientSideColumnEncryptionVersion   ConnectOption = 48 //!< Version of client-side column encryption
	coCompressionLevelAndFlags            ConnectOption = 49 //!< Network compression level and flags (added to hana2sp02)
	coClientSideReExecutionSupported      ConnectOption = 50 //!< support client-side re-execution for client-side encryption (added to hana2sp03)
	coClientReconnectWaitTimeout          ConnectOption = 51 //!< client reconnection wait timeout for transparent session recovery
	coOriginalAnchorConnectionID          ConnectOption = 52 //!< original anchor connectionID to notify client's RECONNECT
	coFlagSet1                            ConnectOption = 53 //!< flags for aggregating several options
	coTopologyNetworkGroup                ConnectOption = 54 //!< NetworkGroup name sent by client to choose topology mapping (added to hana2sp04)
	coIPAddress                           ConnectOption = 55 //!< IP Address of the sender (added to hana2sp04)
	coLRRPingTime                         ConnectOption = 56 //!< Long running request ping time
)
