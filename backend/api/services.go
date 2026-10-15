package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/secrets"
)

// ServiceInspector is the subset of the Docker API needed to inspect a swarm
// service together with its tasks and the nodes they run on.
type ServiceInspector interface {
	ServiceInspectWithRaw(ctx context.Context, serviceID string, opts swarm.ServiceInspectOptions) (swarm.Service, []byte, error)
	TaskList(ctx context.Context, options swarm.TaskListOptions) ([]swarm.Task, error)
	NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error)
	ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error)
}

// ServiceLogger reads the aggregated logs of every task of a swarm service.
// The manager fetches them cluster-wide, so no per-node access is needed.
type ServiceLogger interface {
	ServiceLogs(ctx context.Context, serviceID string, options containertypes.LogsOptions) (io.ReadCloser, error)
}

// ServiceLister lists swarm services (used to enumerate stack log sources).
type ServiceLister interface {
	ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error)
}

// StackServiceAPI groups the swarm calls the aggregate log handlers need.
type StackServiceAPI interface {
	ServiceLister
	ServiceLogger
}

// HandleServiceInspect returns a handler for GET /api/services/{id}. The id
// may be a service name or ID. The response includes the service's tasks.
func HandleServiceInspect(inspector ServiceInspector, networks NetworkInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			jsonError(w, "invalid service ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		svc, _, err := inspector.ServiceInspectWithRaw(ctx, id, swarm.ServiceInspectOptions{})
		if err != nil {
			log.Printf("service inspect %s: %v", id, err)
			// Only a missing service is a 404; a manager error or timeout
			// must not read as "the service is gone".
			if cerrdefs.IsNotFound(err) {
				jsonError(w, "service not found", http.StatusNotFound)
			} else {
				jsonError(w, "service inspect failed", http.StatusBadGateway)
			}
			return
		}

		// Tasks and nodes are best-effort: the service itself is still useful
		// without placement details. A failed list is not an empty one,
		// though: it must not turn into 0 running replicas.
		tasks, err := inspector.TaskList(ctx, swarm.TaskListOptions{
			Filters: filters.NewArgs(filters.Arg("service", svc.ID)),
		})
		tasksOK := err == nil
		if err != nil {
			log.Printf("service tasks %s: %v", id, err)
		}
		nodes, err := inspector.NodeList(ctx, swarm.NodeListOptions{})
		nodesOK := err == nil
		if err != nil {
			log.Printf("service nodes %s: %v", id, err)
		}

		// Job modes have no desired count in the spec, and inspect doesn't
		// carry swarm's own counts: read them like the graph does
		// (ServiceList with Status), so a job isn't shown 0 desired, stopped.
		job := isJobMode(svc.Spec.Mode)
		if job {
			svc.ServiceStatus = serviceStatusCounts(ctx, inspector, svc.ID)
		}

		resp := buildServiceInspectResponse(ctx, svc, tasks, nodes, networks)
		if !tasksOK || !nodesOK || (job && svc.ServiceStatus == nil) {
			fillReplicasFallback(ctx, inspector, svc, resp, tasksOK, nodesOK)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// HandleServiceLogsHistory returns a handler for GET /api/services/{id}/logs/history.
func HandleServiceLogsHistory(logger ServiceLogger) http.HandlerFunc {
	return handleLogsHistory("service", logger.ServiceLogs)
}

// HandleServiceLogs returns a handler for GET /api/services/{id}/logs,
// streaming the logs of all the service's tasks as Server-Sent Events.
func HandleServiceLogs(logger ServiceLogger) http.HandlerFunc {
	return handleLogsStream("service", logger.ServiceLogs)
}

// fillReplicasFallback corrects the replica counts and status of an inspect
// response built from a failed task or node list, or for a job without
// swarm's counts. The running count comes from the tasks, a global service's
// desired count from the nodes and a job's from svc.ServiceStatus; when one
// is missing, swarm's own counts (ServiceList with Status) stand in. If those
// are unavailable too, the status is unknown and replicas null.
func fillReplicasFallback(ctx context.Context, inspector ServiceInspector, svc swarm.Service, resp map[string]any, tasksOK, nodesOK bool) {
	replicas, _ := resp["replicas"].(collector.ReplicaCount)
	runningOK := tasksOK
	// Replicated desired counts come from the spec, job counts from the
	// service status, global counts from the node list.
	var desiredOK bool
	switch {
	case isJobMode(svc.Spec.Mode):
		desiredOK = svc.ServiceStatus != nil
	case svc.Spec.Mode.Global != nil:
		desiredOK = nodesOK
	default:
		desiredOK = true
	}
	if !tasksOK {
		resp["tasksUnavailable"] = true
	}
	if !runningOK || !desiredOK {
		st := svc.ServiceStatus
		if st == nil {
			st = serviceStatusCounts(ctx, inspector, svc.ID)
		}
		if st != nil {
			if !runningOK {
				replicas.Running = int(st.RunningTasks)
				runningOK = true
			}
			if !desiredOK {
				replicas.Desired = int(st.DesiredTasks)
				desiredOK = true
			}
		}
	}
	if !runningOK || !desiredOK {
		resp["replicas"] = nil
		resp[fieldStatus] = collector.ServiceStatusUnknown
		return
	}
	resp["replicas"] = replicas
	resp[fieldStatus] = collector.ServiceStatusOf(svc, replicas)
}

// isJobMode reports whether a service runs to completion (replicated-job or
// global-job): its desired count lives only in swarm's service status.
func isJobMode(m swarm.ServiceMode) bool {
	return m.ReplicatedJob != nil || m.GlobalJob != nil
}

// serviceStatusCounts returns swarm's running/desired task counts for a
// service, or nil when they can't be read.
func serviceStatusCounts(ctx context.Context, lister ServiceLister, serviceID string) *swarm.ServiceStatus {
	services, err := lister.ServiceList(ctx, swarm.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("id", serviceID)),
		Status:  true,
	})
	if err != nil {
		log.Printf("service status %s: %v", serviceID, err)
		return nil
	}
	for _, s := range services {
		if s.ID == serviceID {
			return s.ServiceStatus
		}
	}
	return nil
}

func buildServiceInspectResponse(ctx context.Context, svc swarm.Service, tasks []swarm.Task, nodes []swarm.Node, networks NetworkInspector) map[string]any {
	info, status := collector.SummarizeService(svc, tasks, nodes)

	resp := map[string]any{
		"id":        svc.ID,
		fieldName:   svc.Spec.Name,
		"stack":     collector.ProjectOf(svc.Spec.Labels), // as in the graph
		fieldStatus: status,
		"mode":      info.Mode,
		"replicas":  info.Replicas,
		// Set to true when the task list could not be read.
		"tasksUnavailable": false,
		"tasks":            info.Tasks,
		fieldLabels:        secrets.MaskLabels(svc.Spec.Labels),
		"createdAt":        svc.CreatedAt,
		"updatedAt":        svc.UpdatedAt,
		"ports":            buildServicePorts(svc.Endpoint.Ports),
		"networks":         buildServiceNetworks(ctx, svc, networks),
	}
	if info.Tasks == nil {
		resp["tasks"] = []collector.TaskInfo{}
	}

	if cs := svc.Spec.TaskTemplate.ContainerSpec; cs != nil {
		resp["image"] = cs.Image
		resp["cmd"] = cs.Command
		resp["args"] = cs.Args
		resp["workingDir"] = cs.Dir
		resp["user"] = cs.User
		resp["env"] = filterEnvVars(cs.Env)
		resp["mounts"] = buildServiceMounts(cs.Mounts)
	}
	if p := svc.Spec.TaskTemplate.Placement; p != nil {
		resp["constraints"] = p.Constraints
	}
	if svc.UpdateStatus != nil {
		resp["updateStatus"] = map[string]any{
			"state":   string(svc.UpdateStatus.State),
			"message": svc.UpdateStatus.Message,
		}
	}
	return resp
}

// buildServicePorts lists the service endpoint's ports, published or not.
func buildServicePorts(ports []swarm.PortConfig) []map[string]any {
	result := make([]map[string]any, 0, len(ports))
	for _, p := range ports {
		result = append(result, map[string]any{
			"host":        p.PublishedPort,
			"container":   p.TargetPort,
			"protocol":    string(p.Protocol),
			"publishMode": string(p.PublishMode),
		})
	}
	return result
}

// buildServiceMounts mirrors the container mount payload shape so the same
// detail panel renders both.
func buildServiceMounts(mounts []mount.Mount) []map[string]any {
	result := make([]map[string]any, 0, len(mounts))
	for _, m := range mounts {
		entry := map[string]any{
			"type":        string(m.Type),
			"source":      m.Source,
			"destination": m.Target,
			"rw":          !m.ReadOnly,
		}
		if m.Type == mount.TypeVolume {
			entry[fieldName] = m.Source
		}
		result = append(result, entry)
	}
	return result
}

// buildServiceNetworks resolves the service's network attachments (stored as
// IDs) to names. A network that can't be inspected is reported by its ID.
func buildServiceNetworks(ctx context.Context, svc swarm.Service, networks NetworkInspector) []map[string]any {
	result := make([]map[string]any, 0, len(svc.Spec.TaskTemplate.Networks))
	for _, n := range svc.Spec.TaskTemplate.Networks {
		name := n.Target
		if networks != nil {
			if nw, err := networks.NetworkInspect(ctx, n.Target, networktypes.InspectOptions{}); err == nil && nw.Name != "" {
				name = nw.Name
			}
		}
		result = append(result, map[string]any{
			fieldName: name,
			"aliases": n.Aliases,
		})
	}
	return result
}
