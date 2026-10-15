package collector

// Docker container state values reported by the Docker Engine API.
const (
	// StateRunning is the container State value for a running container.
	// Exported so the api package can use it in disk-usage filtering and
	// inspect-response payloads.
	StateRunning = "running"
)

// Graph node Type field values.
const (
	nodeTypeContainer = "container"
	nodeTypeVolume    = "volume"
	nodeTypeService   = "service"
	nodeTypeSwarmNode = "swarmnode"
)

// Docker mount Type values found on container.Mount entries.
const (
	mountTypeBind   = "bind"
	mountTypeVolume = "volume"
)

// Built-in Docker network names that exist on every host and are not
// part of any user-defined topology. The swarm routing-mesh networks
// (ingress, docker_gwbridge) are plumbing present on every swarm node.
const (
	networkBridge   = "bridge"
	networkIngress  = "ingress"
	networkGwBridge = "docker_gwbridge"
)

// Docker Compose label key for the project a resource belongs to.
const composeProjectLabel = "com.docker.compose.project"

// Swarm label keys: the stack a resource was deployed with (docker stack
// deploy), and the task a container runs for.
const (
	StackNamespaceLabel   = "com.docker.stack.namespace"
	swarmTaskIDLabel      = "com.docker.swarm.task.id"
	swarmServiceNameLabel = "com.docker.swarm.service.name"
)

// Docker event actions that indicate a topology change.
const (
	eventStart        = "start"
	eventStop         = "stop"
	eventDie          = "die"
	eventKill         = "kill"
	eventCreate       = "create"
	eventDestroy      = "destroy"
	eventRename       = "rename"
	eventPause        = "pause"
	eventUnpause      = "unpause"
	eventHealthStatus = "health_status"
	eventConnect      = "connect"
	eventDisconnect   = "disconnect"
	eventUpdate       = "update"
	eventRemove       = "remove"
)

// WireMessage and StateMessage Type field values, shared between the
// collector, state, and api packages so the wire protocol stays in sync.
const (
	MsgTypeSnapshot = "snapshot"
	MsgTypeDelta    = "delta"
)

// Block I/O operation types reported by container stats.
const (
	blkIOOpRead  = "read"
	blkIOOpWrite = "write"
)

// Runtime modes (DG_MODE). Auto resolves to standalone or swarm at startup
// from the daemon's swarm state.
const (
	ModeAuto       = "auto"
	ModeStandalone = "standalone"
	ModeSwarm      = "swarm"
	ModeAgent      = "agent"
)
