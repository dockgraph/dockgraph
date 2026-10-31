package collector

import (
	"context"
	"log"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

// LocateContainer returns the base URL (http://host:port) of the agent on
// the node running container id (an ID, ID prefix, name or task ID), when
// that node is not the local one and has a known agent. Task containers,
// stopped ones included, are placed by the swarm task list, other
// containers by the last stats poll; while the task list can't be read,
// only task containers named or given by task ID with a cached placement
// are located. An id matched on several nodes, or by a task on one node and
// a standalone report on another, is not located.
func (p *AgentPool) LocateContainer(ctx context.Context, id string) (string, bool) {
	// The tasks of the last stats poll, by name or task ID, are the common
	// case, and their placement is cached. A container ID is checked against
	// every task's container instead, so that tasks on two nodes claiming
	// one container leave it unplaced (see ownerFromTasks).
	var nodeID string
	var err error
	if !maybeContainerID(id) {
		nodeID, err = p.taskNode(ctx, p.sampledTask(id), id)
	}
	if nodeID == "" && err == nil {
		nodeID, err = p.ownerFromTasks(ctx, id)
		claimed, standalone := p.standaloneClaims(id)
		// The manager places a task, but a task's container is what its node
		// tells the swarm. Another node reporting that container as a
		// standalone one contradicts it, and nothing tells which is right.
		if nodeID != "" && maybeContainerID(id) && claimed && standalone != nodeID {
			return "", false
		}
		if nodeID == "" && err == nil {
			nodeID = standalone
		}
	}
	return p.agentURL(nodeID)
}

// agentURL returns the base URL of nodeID's agent, unless nodeID is empty,
// the local node or has no known agent.
func (p *AgentPool) agentURL(nodeID string) (string, bool) {
	if nodeID == "" || nodeID == p.cfg.LocalNodeID {
		return "", false
	}
	p.mu.RLock()
	a, ok := p.agents[nodeID]
	p.mu.RUnlock()
	if !ok {
		return "", false
	}
	return "http://" + a.addr, true
}

// sampledTask returns the task of id, a container name or task ID, among
// the task containers of the last remote stats poll: "" when id matched
// none, or when reports disagree on its task.
func (p *AgentPool) sampledTask(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tasks[id]
}

// standaloneClaims looks id up among the standalone containers of the last
// remote stats poll, by container ID, name and ID prefix at once. It
// reports whether any matched, and the node they all name, or "" when they
// disagree: reports are only as good as the agent sending them, so an ID
// one node reports must not take over another node's name or ID prefix.
func (p *AgentPool) standaloneClaims(id string) (claimed bool, nodeID string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var matches []string
	if n, ok := p.owners[id]; ok {
		matches = append(matches, n)
	}
	if n, ok := p.names[id]; ok {
		matches = append(matches, n)
	}
	if n, ok := uniquePrefixMatch(p.owners, id); ok {
		matches = append(matches, n)
	}
	if len(matches) == 0 {
		return false, ""
	}
	if slices.ContainsFunc(matches, func(n string) bool { return n != matches[0] }) {
		return true, ""
	}
	return true, matches[0]
}

// uniquePrefixMatch returns the value of the keys that id, when it can be a
// container ID prefix, is a prefix of. found is true when any key matched;
// the value is "" when matching keys hold different values.
func uniquePrefixMatch(values map[string]string, id string) (value string, found bool) {
	if !maybeContainerID(id) {
		return "", false
	}
	for key, v := range values {
		if !strings.HasPrefix(key, id) {
			continue
		}
		if found && v != value {
			return "", true
		}
		value, found = v, true
	}
	return value, found
}

// taskNode returns the node the swarm runs task taskID on, or "" when the
// swarm has no such task or id, from whichever field of an agent's report,
// is neither taskID nor a task container name ending in it. err is set when
// the task list could not be read.
func (p *AgentPool) taskNode(ctx context.Context, taskID, id string) (string, error) {
	if taskID == "" || p.cfg.Tasks == nil || taskIDOfName(id) != taskID {
		return "", nil
	}
	return p.placement(ctx, taskID, id)
}

// placement returns the node the swarm runs taskID on, from the cache or
// the task list, or "" when the swarm has no such task. Only the placement
// of a task whose container is known is cached.
func (p *AgentPool) placement(ctx context.Context, taskID, id string) (string, error) {
	p.mu.RLock()
	cached, ok := p.placements[taskID]
	p.mu.RUnlock()
	if ok {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(ctx, agentTaskLookupTimeout)
	defer cancel()
	tasks, err := p.cfg.Tasks.TaskList(ctx, swarm.TaskListOptions{Filters: filters.NewArgs(filters.Arg("id", taskID))})
	if err != nil {
		log.Printf("agents: task lookup for %s: %v", id, err)
		return "", err
	}
	for _, t := range tasks {
		if t.ID != taskID {
			continue
		}
		if cs := t.Status.ContainerStatus; cs != nil && cs.ContainerID != "" {
			p.mu.Lock()
			if _, sampled := p.tasks[taskID]; sampled {
				p.placements[taskID] = t.NodeID
			}
			p.mu.Unlock()
		}
		return t.NodeID, nil
	}
	return "", nil
}

// ownerFromTasks finds the task running container id through the swarm
// task list, and returns its node, or "" when the swarm runs no task with
// that container, or tasks on several nodes claim it (a task's container is
// what its node reports). Only an id that may be a container ID needs the
// whole list; any other is filtered down to its task ID. err is set when
// the task list could not be read.
func (p *AgentPool) ownerFromTasks(ctx context.Context, id string) (string, error) {
	if p.cfg.Tasks == nil {
		return "", nil
	}
	taskID := taskIDOfName(id)
	if taskID == "" {
		return "", nil
	}
	var opts swarm.TaskListOptions
	if !maybeContainerID(id) {
		opts.Filters = filters.NewArgs(filters.Arg("id", taskID))
	}
	ctx, cancel := context.WithTimeout(ctx, agentTaskLookupTimeout)
	defer cancel()
	tasks, err := p.cfg.Tasks.TaskList(ctx, opts)
	if err != nil {
		log.Printf("agents: task lookup for %s: %v", id, err)
		return "", err
	}
	owner := ""
	for _, t := range tasks {
		if t.ID == taskID {
			return t.NodeID, nil
		}
		if cs := t.Status.ContainerStatus; cs != nil && cs.ContainerID != "" {
			if cs.ContainerID == id || (len(id) >= 12 && strings.HasPrefix(cs.ContainerID, id)) {
				if owner != "" && owner != t.NodeID {
					return "", nil
				}
				owner = t.NodeID
			}
		}
	}
	return owner, nil
}

// taskIDOfName returns the task ID a task container name ends in: swarm
// names task containers {service}.{slot|node}.{taskID}. A task ID or a
// container ID is returned as it is.
func taskIDOfName(name string) string {
	return name[strings.LastIndexByte(name, '.')+1:]
}

// maybeContainerID reports whether id can be a container ID or an ID prefix
// long enough to match one (12 to 64 hex digits).
func maybeContainerID(id string) bool {
	if len(id) < 12 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
