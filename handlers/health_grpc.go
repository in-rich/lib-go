package handlers

import (
	"context"
	common_pb "github.com/in-rich/proto/proto-go/common"
)

type ServiceCheck func() bool

func DummyServiceCheck() bool {
	return true
}

type HealthHandler struct {
	common_pb.HealthServer
	services map[string]ServiceCheck
}

func (h *HealthHandler) Check(_ context.Context, in *common_pb.HealthCheckRequest) (*common_pb.HealthCheckResponse, error) {
	if in.Service != "" {
		checkFn, ok := h.services[in.Service]

		if !ok {
			return &common_pb.HealthCheckResponse{
				Status: common_pb.HealthCheckResponse_UNKNOWN,
			}, nil
		}

		if !checkFn() {
			return &common_pb.HealthCheckResponse{
				Status: common_pb.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		return &common_pb.HealthCheckResponse{
			Status: common_pb.HealthCheckResponse_SERVING,
		}, nil
	}

	return &common_pb.HealthCheckResponse{
		Status: common_pb.HealthCheckResponse_SERVING,
	}, nil
}

func (h *HealthHandler) Watch(in *common_pb.HealthCheckRequest, stream common_pb.Health_WatchServer) error {

	return nil
}
