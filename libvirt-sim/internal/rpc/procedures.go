package rpc

// Libvirt remote procedure numbers.
// These constants match the libvirt RPC protocol specification
// as defined in github.com/digitalocean/go-libvirt internal/constants/remote_protocol.gen.go.
const (
	// ProcConnectOpen is REMOTE_PROC_CONNECT_OPEN.
	ProcConnectOpen int32 = 1
	// ProcConnectClose is REMOTE_PROC_CONNECT_CLOSE.
	ProcConnectClose int32 = 2
	// ProcConnectGetVersion is REMOTE_PROC_CONNECT_GET_VERSION.
	ProcConnectGetVersion int32 = 4
	// ProcNodeGetInfo is REMOTE_PROC_NODE_GET_INFO.
	ProcNodeGetInfo int32 = 6
	// ProcConnectGetCapabilities is REMOTE_PROC_CONNECT_GET_CAPABILITIES.
	ProcConnectGetCapabilities int32 = 7
	// ProcDomainGetXMLDesc is REMOTE_PROC_DOMAIN_GET_XML_DESC.
	ProcDomainGetXMLDesc int32 = 14
	// ProcDomainGetInfo is REMOTE_PROC_DOMAIN_GET_INFO.
	ProcDomainGetInfo int32 = 16
	// ProcDomainLookupByName is REMOTE_PROC_DOMAIN_LOOKUP_BY_NAME.
	ProcDomainLookupByName int32 = 23
	// ProcDomainLookupByUUID is REMOTE_PROC_DOMAIN_LOOKUP_BY_UUID.
	ProcDomainLookupByUUID int32 = 24
	// ProcDomainReboot is REMOTE_PROC_DOMAIN_REBOOT.
	ProcDomainReboot int32 = 27
	// ProcDomainResume is REMOTE_PROC_DOMAIN_RESUME.
	ProcDomainResume int32 = 28
	// ProcDomainSuspend is REMOTE_PROC_DOMAIN_SUSPEND.
	ProcDomainSuspend int32 = 34
	// ProcConnectGetHostname is REMOTE_PROC_CONNECT_GET_HOSTNAME.
	ProcConnectGetHostname int32 = 59
	// ProcNodeGetFreeMemory is REMOTE_PROC_NODE_GET_FREE_MEMORY.
	ProcNodeGetFreeMemory int32 = 102
	// ProcDomainCreateWithFlags is REMOTE_PROC_DOMAIN_CREATE_WITH_FLAGS.
	ProcDomainCreateWithFlags int32 = 196
	// ProcDomainGetState is REMOTE_PROC_DOMAIN_GET_STATE.
	ProcDomainGetState int32 = 212
	// ProcNodeGetCPUStats is REMOTE_PROC_NODE_GET_CPU_STATS.
	ProcNodeGetCPUStats int32 = 227
	// ProcNodeGetMemoryStats is REMOTE_PROC_NODE_GET_MEMORY_STATS.
	ProcNodeGetMemoryStats int32 = 228
	// ProcDomainUndefineFlags is REMOTE_PROC_DOMAIN_UNDEFINE_FLAGS.
	ProcDomainUndefineFlags int32 = 231
	// ProcDomainDestroyFlags is REMOTE_PROC_DOMAIN_DESTROY_FLAGS.
	ProcDomainDestroyFlags int32 = 234
	// ProcDomainShutdownFlags is REMOTE_PROC_DOMAIN_SHUTDOWN_FLAGS.
	ProcDomainShutdownFlags int32 = 258
	// ProcDomainListAllDomains is REMOTE_PROC_CONNECT_LIST_ALL_DOMAINS.
	ProcDomainListAllDomains int32 = 273
	// ProcConnectGetAllDomainStats is REMOTE_PROC_CONNECT_GET_ALL_DOMAIN_STATS.
	ProcConnectGetAllDomainStats int32 = 344
	// ProcDomainDefineXMLFlags is REMOTE_PROC_DOMAIN_DEFINE_XML_FLAGS.
	ProcDomainDefineXMLFlags int32 = 350
)

// Libvirt error codes.
const (
	// VirErrNoDomain indicates the domain was not found.
	VirErrNoDomain int32 = 42
	// VirErrOperationInvalid indicates the operation is not valid.
	VirErrOperationInvalid int32 = 55
	// VirErrOperationDenied indicates the operation was denied.
	VirErrOperationDenied int32 = 6
	// VirErrInternalError indicates an internal error.
	VirErrInternalError int32 = 1
)

// Libvirt error domains.
const (
	// VirFromQemu is the QEMU error domain.
	VirFromQemu int32 = 18
)
