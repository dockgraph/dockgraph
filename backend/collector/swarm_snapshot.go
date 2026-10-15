package collector

import (
	"context"
	"hash/fnv"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	volumetypes "github.com/docker/docker/api/types/volume"
	"golang.org/x/sync/errgroup"
)

// Service display statuses. "running" reuses StateRunning.
const (
	serviceStatusDegraded = "degraded"
	serviceStatusStopped  = "stopped"
	serviceStatusUpdating = "updating"
)

// Service mode names reported in ServiceInfo.Mode.
const (
	serviceModeReplicated    = "replicated"
	serviceModeGlobal        = "global"
	serviceModeReplicatedJob = "replicated-job"
	serviceModeGlobalJob     = "global-job"
)

// maxFailedTasks caps how many recent failed/rejected tasks are listed per
// service alongside its active tasks, so a crash-looping service doesn't
// flood the node payload with its whole task history.
const maxFailedTasks = 3

// maxCompletedTasks caps how many recently completed job tasks are listed per
// service. Job tasks keep desired state "complete" after they finish, and
// swarm retains them, so a large or often rerun job would otherwise send its
// whole completion history in every snapshot.
const maxCompletedTasks = 3

// swarmResources extends the local daemon resources with the cluster-wide
// swarm objects visible from a manager node.
type swarmResources struct {
	dockerResources
	services []swarm.Service
	tasks    []swarm.Task
	nodes    []swarm.Node
}

// fetchSwarmResources queries the local resources plus swarm services, tasks
// and nodes concurrently. If any call fails, the context is cancelled and the
// first error is returned.
func fetchSwarmResources(ctx context.Context, cli DockerClient) (swarmResources, error) {
	var res swarmResources
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		res.containers, err = cli.ContainerList(ctx, containertypes.ListOptions{All: true})
		return err
	})
	g.Go(func() error {
		var err error
		res.networks, err = cli.NetworkList(ctx, networktypes.ListOptions{})
		return err
	})
	g.Go(func() error {
		volResp, err := cli.VolumeList(ctx, volumetypes.ListOptions{})
		if err != nil {
			return err
		}
		res.volumes = volResp.Volumes
		return nil
	})
	g.Go(func() error {
		var err error
		res.services, err = cli.ServiceList(ctx, swarm.ServiceListOptions{Status: true})
		return err
	})
	g.Go(func() error {
		var err error
		res.tasks, err = cli.TaskList(ctx, swarm.TaskListOptions{})
		return err
	})
	g.Go(func() error {
		var err error
		res.nodes, err = cli.NodeList(ctx, swarm.NodeListOptions{})
		return err
	})

	if err := g.Wait(); err != nil {
		return swarmResources{}, err
	}
	return res, nil
}

// IsTaskContainer reports whether a container runs a swarm task. Such
// containers are represented by their service node instead.
func IsTaskContainer(labels map[string]string) bool {
	return labels[swarmTaskIDLabel] != ""
}

// buildSwarmSnapshot assembles the cluster graph: one node per swarm service
// (stable across task reschedules), the overlay and local networks they join,
// the volumes they mount, and any standalone (non-task) containers running on
// the local daemon.
func buildSwarmSnapshot(res swarmResources) GraphSnapshot {
	local := res.dockerResources
	local.containers = make([]containertypes.Summary, 0, len(res.containers))
	for _, c := range res.containers {
		if !IsTaskContainer(c.Labels) {
			local.containers = append(local.containers, c)
		}
	}

	// Stack membership counts standalone containers and services (which stand
	// for their task containers, whose labels may lack the service-level
	// self-exclusion label), so a stack whose only service is DockGraph has
	// its networks and volumes hidden.
	members := containerMembers(local.containers)
	for _, svc := range res.services {
		members = append(members, projectMember{ProjectOf(svc.Spec.Labels), IsServiceSelfExcluded(svc)})
	}
	snap := assembleSnapshot(local, selfOnlyProjects(members))

	knownVolumes := make(map[string]bool)
	for _, n := range snap.Nodes {
		if n.Type == nodeTypeVolume {
			knownVolumes[n.Name] = true
		}
	}

	ctx := serviceContext{
		networkIDToName: resolveNetworkNames(res.networks),
		networkProjects: networkProjectMap(res.networks),
		hostnames:       nodeHostnames(res.nodes),
		tasks:           groupTasksByService(res.tasks),
		eligibleNodes:   countEligibleNodes(res.nodes),
	}

	for _, svc := range res.services {
		if IsServiceSelfExcluded(svc) {
			continue
		}
		node, edges := buildSwarmServiceNode(svc, ctx)
		snap.Nodes = append(snap.Nodes, node)
		snap.Edges = append(snap.Edges, edges...)

		// Swarm volumes are node-local, so a service's volume may not exist on
		// this daemon; add a node for it so the mount edge has a target.
		for _, m := range serviceVolumeMounts(svc) {
			if knownVolumes[m.Source] {
				continue
			}
			knownVolumes[m.Source] = true
			driver := "local"
			if m.VolumeOptions != nil && m.VolumeOptions.DriverConfig != nil && m.VolumeOptions.DriverConfig.Name != "" {
				driver = m.VolumeOptions.DriverConfig.Name
			}
			vol := buildVolumeNode(m.Source, driver, "created")
			// Stack deploy names its volumes {stack}_{name}; anything else is
			// an external volume that doesn't belong to the stack.
			if node.Stack != "" && strings.HasPrefix(m.Source, node.Stack+"_") {
				vol.Stack = node.Stack
			}
			snap.Nodes = append(snap.Nodes, vol)
		}
	}

	snap.Nodes = append(snap.Nodes, buildSwarmNodeNodes(res.nodes)...)

	sortSnapshot(&snap)
	return snap
}

// buildSwarmNodeNodes creates one "swarmnode" graph node per cluster node,
// keyed by hostname (the name tasks report via TaskInfo.NodeHostname and the
// key of per-node stats aggregates). A node that left and rejoined keeps a
// stale entry with the same hostname; when hostnames collide the ready node
// wins, then the most recently updated one, so each hostname maps to a single
// graph node. A node without a hostname falls back to its ID. DockGraph's own
// nodes are infrastructure and never self-excluded.
func buildSwarmNodeNodes(nodes []swarm.Node) []Node {
	chosen := make(map[string]swarm.Node, len(nodes))
	for _, n := range nodes {
		name := swarmNodeName(n)
		prev, ok := chosen[name]
		if ok && prev.Status.State == swarm.NodeStateReady && n.Status.State == swarm.NodeStateReady {
			// Not a stale entry of a rejoined node: two live members.
			warnOnce("node-hostname:"+name, "swarm: nodes %s and %s are both ready with hostname %q; "+
				"DockGraph keys nodes by hostname and shows only one (see README, Known limitations)", prev.ID, n.ID, name)
		}
		if ok && !preferSwarmNode(n, prev) {
			continue
		}
		chosen[name] = n
	}
	out := make([]Node, 0, len(chosen))
	for name, n := range chosen {
		out = append(out, buildSwarmNodeNode(name, n))
	}
	return out
}

// swarmNodeName is the hostname a swarm node is displayed and keyed under.
func swarmNodeName(n swarm.Node) string {
	if n.Description.Hostname != "" {
		return n.Description.Hostname
	}
	return n.ID
}

// preferSwarmNode reports whether a should replace b for the same hostname.
func preferSwarmNode(a, b swarm.Node) bool {
	aReady := a.Status.State == swarm.NodeStateReady
	bReady := b.Status.State == swarm.NodeStateReady
	if aReady != bReady {
		return aReady
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	return a.ID < b.ID
}

// buildSwarmNodeNode converts a swarm node into its graph node.
func buildSwarmNodeNode(name string, n swarm.Node) Node {
	state := string(n.Status.State)
	if state == "" {
		state = string(swarm.NodeStateUnknown)
	}
	info := &SwarmNodeInfo{
		ID:            n.ID,
		Role:          string(n.Spec.Role),
		Leader:        n.ManagerStatus != nil && n.ManagerStatus.Leader,
		Availability:  string(n.Spec.Availability),
		State:         state,
		Addr:          n.Status.Addr,
		EngineVersion: n.Description.Engine.EngineVersion,
		NanoCPUs:      n.Description.Resources.NanoCPUs,
		MemoryBytes:   n.Description.Resources.MemoryBytes,
	}
	node := Node{
		ID:        "swarmnode:" + name,
		Type:      nodeTypeSwarmNode,
		Name:      name,
		Status:    state,
		SwarmNode: info,
	}
	if !n.CreatedAt.IsZero() {
		node.CreatedAt = n.CreatedAt.UTC().Format(time.RFC3339)
	}
	return node
}

// serviceContext holds cluster-wide lookups shared by every service node.
type serviceContext struct {
	networkIDToName map[string]string
	networkProjects map[string]string
	hostnames       map[string]string
	tasks           map[string][]swarm.Task
	eligibleNodes   int
}

// buildSwarmServiceNode creates the graph node for one swarm service along
// with its secondary-network and volume-mount edges.
func buildSwarmServiceNode(svc swarm.Service, sc serviceContext) (Node, []Edge) {
	name := svc.Spec.Name
	nodeID := "service:" + name
	stack := ProjectOf(svc.Spec.Labels)

	image := ""
	if cs := svc.Spec.TaskTemplate.ContainerSpec; cs != nil {
		image = stripImageDigest(cs.Image)
	}

	info := buildServiceInfo(svc, sc.tasks[svc.ID], sc.hostnames, sc.eligibleNodes)

	node := Node{
		ID:      nodeID,
		Type:    nodeTypeService,
		Name:    name,
		Image:   image,
		Status:  serviceStatus(info.Replicas.Running, info.Replicas.Desired, svc.UpdateStatus),
		Ports:   servicePorts(svc.Endpoint.Ports),
		Labels:  projectLabels(svc.Spec.Labels),
		Stack:   stack,
		Service: info,
	}
	if !svc.CreatedAt.IsZero() {
		node.CreatedAt = svc.CreatedAt.UTC().Format(time.RFC3339)
	}

	primary, secondary := classifyNetworks(serviceNetworks(svc, sc.networkIDToName), stack, sc.networkProjects)
	if primary != "" {
		node.NetworkID = "network:" + primary
	}

	var edges []Edge
	for _, netName := range secondary {
		edges = append(edges, Edge{
			ID:     "e:net:" + name + ":" + netName,
			Type:   "secondary_network",
			Source: nodeID,
			Target: "network:" + netName,
		})
	}
	for _, m := range serviceVolumeMounts(svc) {
		edges = append(edges, Edge{
			ID:        "e:vol:" + m.Source + ":" + name,
			Type:      "volume_mount",
			Source:    "volume:" + m.Source,
			Target:    nodeID,
			MountPath: m.Target,
		})
	}

	return node, edges
}

// serviceNetworks resolves a service's network attachments to tracked network
// names. Attachments normally reference networks by ID; a name is accepted
// too. When the task template lists none, the endpoint's virtual IPs are used.
// Built-in and routing-mesh networks are absent from networkIDToName and so
// are dropped.
func serviceNetworks(svc swarm.Service, networkIDToName map[string]string) []string {
	var targets []string
	for _, n := range svc.Spec.TaskTemplate.Networks {
		targets = append(targets, n.Target)
	}
	if len(targets) == 0 {
		for _, vip := range svc.Endpoint.VirtualIPs {
			targets = append(targets, vip.NetworkID)
		}
	}

	names := make(map[string]bool, len(networkIDToName))
	for _, n := range networkIDToName {
		names[n] = true
	}

	seen := make(map[string]bool)
	var result []string
	for _, t := range targets {
		name, ok := networkIDToName[t]
		if !ok && names[t] {
			name, ok = t, true
		}
		if ok && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

// serviceVolumeMounts returns the named-volume mounts of a service's container spec.
func serviceVolumeMounts(svc swarm.Service) []mount.Mount {
	cs := svc.Spec.TaskTemplate.ContainerSpec
	if cs == nil {
		return nil
	}
	var mounts []mount.Mount
	for _, m := range cs.Mounts {
		if m.Type == mount.TypeVolume && m.Source != "" {
			mounts = append(mounts, m)
		}
	}
	return mounts
}

// servicePorts converts the service endpoint's published ports into the
// wire-format representation. Unpublished (target-only) ports are skipped.
func servicePorts(ports []swarm.PortConfig) []PortMapping {
	var result []PortMapping
	for _, p := range ports {
		if p.PublishedPort == 0 {
			continue
		}
		result = append(result, PortMapping{
			Host:      int(p.PublishedPort),
			Container: int(p.TargetPort),
			Protocol:  string(p.Protocol),
		})
	}
	return result
}

// stripImageDigest drops the "@sha256:..." pin that swarm appends to service
// images, keeping the human-readable reference.
func stripImageDigest(image string) string {
	ref, _, _ := strings.Cut(image, "@")
	return ref
}

// buildServiceInfo derives mode, replica counts, update state and the visible
// task list for a service.
func buildServiceInfo(svc swarm.Service, tasks []swarm.Task, hostnames map[string]string, eligibleNodes int) *ServiceInfo {
	info := &ServiceInfo{Mode: serviceMode(svc.Spec.Mode)}
	if svc.UpdateStatus != nil {
		info.UpdateStatus = string(svc.UpdateStatus.State)
	}

	var active, completed, failed []swarm.Task
	for _, t := range tasks {
		switch {
		case t.DesiredState == swarm.TaskStateShutdown || t.DesiredState == swarm.TaskStateRemove:
			if t.Status.State == swarm.TaskStateFailed || t.Status.State == swarm.TaskStateRejected {
				failed = append(failed, t)
			}
		case t.Status.State == swarm.TaskStateComplete:
			// A finished job task: history, not current work.
			completed = append(completed, t)
		default:
			active = append(active, t)
			if t.Status.State == swarm.TaskStateRunning {
				info.Replicas.Running++
			}
		}
	}

	info.Replicas.Desired = desiredReplicas(svc, eligibleNodes)

	// Keep only the most recent history.
	completed = mostRecentTasks(completed, maxCompletedTasks)
	failed = mostRecentTasks(failed, maxFailedTasks)

	for _, t := range slices.Concat(active, completed, failed) {
		info.Tasks = append(info.Tasks, buildTaskInfo(t, hostnames))
	}
	sort.SliceStable(info.Tasks, func(i, j int) bool {
		a, b := info.Tasks[i], info.Tasks[j]
		if a.Slot != b.Slot {
			return a.Slot < b.Slot
		}
		if a.NodeHostname != b.NodeHostname {
			return a.NodeHostname < b.NodeHostname
		}
		return a.ID < b.ID
	})
	return info
}

// mostRecentTasks returns the n tasks with the latest status timestamps.
func mostRecentTasks(tasks []swarm.Task, n int) []swarm.Task {
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Status.Timestamp.After(tasks[j].Status.Timestamp) })
	if len(tasks) > n {
		tasks = tasks[:n]
	}
	return tasks
}

// buildTaskInfo converts a swarm task into its wire representation.
func buildTaskInfo(t swarm.Task, hostnames map[string]string) TaskInfo {
	ti := TaskInfo{
		ID:           t.ID,
		Slot:         t.Slot,
		NodeID:       t.NodeID,
		NodeHostname: hostnames[t.NodeID],
		State:        string(t.Status.State),
		DesiredState: string(t.DesiredState),
		Error:        t.Status.Err,
	}
	if t.Status.ContainerStatus != nil {
		ti.ContainerID = t.Status.ContainerStatus.ContainerID
	}
	if !t.Status.Timestamp.IsZero() {
		ti.Timestamp = t.Status.Timestamp.UTC().Format(time.RFC3339)
	}
	return ti
}

// serviceMode names the scheduling mode of a service.
func serviceMode(m swarm.ServiceMode) string {
	switch {
	case m.Global != nil:
		return serviceModeGlobal
	case m.ReplicatedJob != nil:
		return serviceModeReplicatedJob
	case m.GlobalJob != nil:
		return serviceModeGlobalJob
	default:
		return serviceModeReplicated
	}
}

// desiredReplicas returns how many tasks a service should be running. For
// replicated services this is the configured replica count. Global and job
// services use the manager-computed ServiceStatus when available; global
// services otherwise fall back to the number of schedulable nodes.
func desiredReplicas(svc swarm.Service, eligibleNodes int) int {
	if r := svc.Spec.Mode.Replicated; r != nil {
		if r.Replicas == nil {
			return 1
		}
		return int(*r.Replicas)
	}
	if svc.ServiceStatus != nil {
		return int(svc.ServiceStatus.DesiredTasks)
	}
	if svc.Spec.Mode.Global != nil {
		return eligibleNodes
	}
	return 0
}

// serviceStatus derives a display status from replica counts and the
// rolling-update state: "updating" while an update or rollback is in flight,
// "stopped" when scaled to zero, "running" when all replicas are up and
// "degraded" otherwise.
func serviceStatus(running, desired int, update *swarm.UpdateStatus) string {
	if update != nil && (update.State == swarm.UpdateStateUpdating || update.State == swarm.UpdateStateRollbackStarted) {
		return serviceStatusUpdating
	}
	if desired == 0 {
		return serviceStatusStopped
	}
	if running >= desired {
		return StateRunning
	}
	return serviceStatusDegraded
}

// nodeHostnames maps swarm node IDs to their hostnames.
func nodeHostnames(nodes []swarm.Node) map[string]string {
	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n.Description.Hostname
	}
	return m
}

// countEligibleNodes counts nodes that are ready and accept tasks, which is
// where a global service schedules one task each.
func countEligibleNodes(nodes []swarm.Node) int {
	count := 0
	for _, n := range nodes {
		if n.Status.State == swarm.NodeStateReady && n.Spec.Availability == swarm.NodeAvailabilityActive {
			count++
		}
	}
	return count
}

// groupTasksByService indexes tasks by their service ID.
func groupTasksByService(tasks []swarm.Task) map[string][]swarm.Task {
	m := make(map[string][]swarm.Task)
	for _, t := range tasks {
		m[t.ServiceID] = append(m[t.ServiceID], t)
	}
	return m
}

// taskFingerprint hashes the identity, placement and state of every task,
// plus the identity, state, availability and role of every cluster node.
// Task transitions on remote nodes emit no events on the manager, and a node
// going down is not reliably evented either, so the collector polls TaskList
// and NodeList and re-snapshots only when this value changes.
func taskFingerprint(tasks []swarm.Task, nodes []swarm.Node) uint64 {
	keys := make([]string, 0, len(tasks)+len(nodes))
	for _, t := range tasks {
		keys = append(keys, "t|"+t.ID+"|"+t.NodeID+"|"+string(t.Status.State)+"|"+string(t.DesiredState))
	}
	for _, n := range nodes {
		leader := n.ManagerStatus != nil && n.ManagerStatus.Leader
		keys = append(keys, "n|"+n.ID+"|"+n.Description.Hostname+"|"+string(n.Status.State)+"|"+
			string(n.Spec.Availability)+"|"+string(n.Spec.Role)+"|"+strconv.FormatBool(leader))
	}
	sort.Strings(keys)
	h := fnv.New64a()
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// SummarizeService computes the ServiceInfo and display status of a single
// service from its tasks and the cluster's nodes, for callers (such as the
// service inspect API) outside the snapshot path.
func SummarizeService(svc swarm.Service, tasks []swarm.Task, nodes []swarm.Node) (*ServiceInfo, string) {
	info := buildServiceInfo(svc, tasks, nodeHostnames(nodes), countEligibleNodes(nodes))
	return info, ServiceStatusOf(svc, info.Replicas)
}

// ServiceStatusOf derives a service's display status from its replica counts
// and rolling-update state.
func ServiceStatusOf(svc swarm.Service, replicas ReplicaCount) string {
	return serviceStatus(replicas.Running, replicas.Desired, svc.UpdateStatus)
}

// ServiceStatusUnknown is the status of a service whose replica counts could
// not be determined.
const ServiceStatusUnknown = "unknown"

// swarmServiceIDLabel is set on task containers to their service's ID.
const swarmServiceIDLabel = "com.docker.swarm.service.id"

// SelfServices remembers which swarm services are DockGraph itself. Service
// events carry only the service name, not its labels, and task container
// events lack the service-level (deploy) labels, so the event history
// resolves self-exclusion by service ID through this cache.
type SelfServices struct {
	inspect func(ctx context.Context, id string) (swarm.Service, error)
	now     func() time.Time

	mu    sync.Mutex
	known map[string]selfEntry
}

// selfEntry is a cached answer. A zero expires never expires.
type selfEntry struct {
	self    bool
	expires time.Time
}

// Bounds of the SelfServices cache. An inspect failing for another reason
// than "not found" is retried only after selfRetryAfter, so a struggling
// daemon doesn't stall every event on a new lookup. The cache starts over
// once it holds selfCacheMax services.
const (
	selfRetryAfter = 30 * time.Second
	selfCacheMax   = 1024
)

// NewSelfServices creates a cache that looks unknown services up via inspect.
func NewSelfServices(inspect func(ctx context.Context, id string) (swarm.Service, error)) *SelfServices {
	return &SelfServices{inspect: inspect, now: time.Now, known: make(map[string]selfEntry)}
}

// IsSelfEvent reports whether a service event, or a task container event,
// belongs to a self-excluded DockGraph service.
func (s *SelfServices) IsSelfEvent(ctx context.Context, msg events.Message) bool {
	switch msg.Type {
	case events.ServiceEventType:
		if msg.Action == events.ActionRemove {
			return s.forget(msg.Actor.ID)
		}
		return s.isSelf(ctx, msg.Actor.ID)
	case events.ContainerEventType:
		if id := msg.Actor.Attributes[swarmServiceIDLabel]; id != "" {
			return s.isSelf(ctx, id)
		}
	}
	return false
}

// forget drops a removed service from the cache and returns its last known
// answer. The service is gone, so there is nothing left to inspect.
func (s *SelfServices) forget(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.known[id]
	delete(s.known, id)
	return e.self
}

// isSelf looks a service up once and caches the answer. A failed inspect
// counts as not self: "not found" (a removed service) is cached for good,
// any other error only for selfRetryAfter.
func (s *SelfServices) isSelf(ctx context.Context, id string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	e, ok := s.known[id]
	s.mu.Unlock()
	if ok && (e.expires.IsZero() || s.now().Before(e.expires)) {
		return e.self
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	svc, err := s.inspect(ctx, id)
	switch {
	case err == nil:
		e = selfEntry{self: IsServiceSelfExcluded(svc)}
	case cerrdefs.IsNotFound(err):
		e = selfEntry{}
	default:
		e = selfEntry{expires: s.now().Add(selfRetryAfter)}
	}
	s.mu.Lock()
	if len(s.known) >= selfCacheMax {
		clear(s.known)
	}
	s.known[id] = e
	s.mu.Unlock()
	return e.self
}
