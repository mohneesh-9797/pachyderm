package worker

import (
	"context"
	"fmt"
	"path"

	"github.com/gogo/protobuf/types"
	"github.com/pachyderm/pachyderm/src/client"
	"github.com/pachyderm/pachyderm/src/client/pps"

	etcd "github.com/coreos/etcd/clientv3"
	"google.golang.org/grpc"
)

const (
	workerEtcdPrefix = "workers"
)

// Status returns the statuses of workers referenced by pipelineRcName.
// pipelineRcName is the name of the pipeline's RC and can be gotten with
// ppsutil.PipelineRcName. You can also pass "" for pipelineRcName to get all
// clients for all workers.
func Status(ctx context.Context, pipelineRcName string, etcdClient *etcd.Client, etcdPrefix string) ([]*pps.WorkerStatus, error) {
	workerClients, err := WorkerClients(ctx, pipelineRcName, etcdClient, etcdPrefix)
	if err != nil {
		return nil, err
	}
	var result []*pps.WorkerStatus
	for _, workerClient := range workerClients {
		status, err := workerClient.Status(ctx, &types.Empty{})
		if err != nil {
			return nil, err
		}
		result = append(result, status)
	}
	return result, nil
}

// Cancel cancels a set of datums running on workers.
// pipelineRcName is the name of the pipeline's RC and can be gotten with
// ppsutil.PipelineRcName.
func Cancel(ctx context.Context, pipelineRcName string, etcdClient *etcd.Client,
	etcdPrefix string, jobID string, dataFilter []string) error {
	workerClients, err := WorkerClients(ctx, pipelineRcName, etcdClient, etcdPrefix)
	if err != nil {
		return err
	}
	success := false
	for _, workerClient := range workerClients {
		resp, err := workerClient.Cancel(ctx, &CancelRequest{
			JobID:       jobID,
			DataFilters: dataFilter,
		})
		if err != nil {
			return err
		}
		if resp.Success {
			success = true
		}
	}
	if !success {
		return fmt.Errorf("datum matching filter %+v could not be found for jobID %s", dataFilter, jobID)
	}
	return nil
}

// WorkerClients returns a slice of worker clients for a pipeline.
// pipelineRcName is the name of the pipeline's RC and can be gotten with
// ppsutil.PipelineRcName. You can also pass "" for pipelineRcName to get all
// clients for all workers.
func WorkerClients(ctx context.Context, pipelineRcName string, etcdClient *etcd.Client, etcdPrefix string) ([]WorkerClient, error) {
	resp, err := etcdClient.Get(ctx, path.Join(etcdPrefix, workerEtcdPrefix, pipelineRcName), etcd.WithPrefix())
	if err != nil {
		return nil, err
	}

	var result []WorkerClient
	for _, kv := range resp.Kvs {
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", path.Base(string(kv.Key)), client.PPSWorkerPort),
			append(client.DefaultDialOptions(), grpc.WithInsecure())...)
		if err != nil {
			return nil, err
		}
		result = append(result, NewWorkerClient(conn))
	}
	return result, nil
}
