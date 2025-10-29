package grpcsvc

import (
	"context"
	"net"
	"strings"

	"github.com/Heidric/metrics.git/internal/model"
	"github.com/Heidric/metrics.git/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Metrics interface {
	ListMetrics() map[string]string
	GetMetric(metricType, metricName string) (string, error)
	UpdateGauge(name, value string) error
	UpdateCounter(name, value string) error
	UpdateMetricJSON(metric *model.Metrics) error
	GetMetricJSON(metric *model.Metrics) error
	UpdateMetricsBatch(metrics []*model.Metrics) error
	Ping(ctx context.Context) error
}

type Server struct {
	pb.UnimplementedMetricsServiceServer
	m       Metrics
	trusted *net.IPNet // nil => без ограничений
}

func New(m Metrics, trusted *net.IPNet) *Server {
	return &Server{m: m, trusted: trusted}
}

func trustedUnary(trusted *net.IPNet) grpc.UnaryServerInterceptor {
	if trusted == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var ipStr string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			v := md.Get("x-real-ip")
			if len(v) > 0 {
				ipStr = strings.TrimSpace(v[0])
			}
		}
		if ipStr == "" {
			if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
				host, _, _ := net.SplitHostPort(p.Addr.String())
				ipStr = host
			}
		}
		ip := net.ParseIP(ipStr)
		if ip == nil || !trusted.Contains(toV4ifNeeded(trusted, ip)) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}
		return handler(ctx, req)
	}
}

func toV4ifNeeded(n *net.IPNet, ip net.IP) net.IP {
	if n == nil {
		return ip
	}
	if n.IP.To4() != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4
		}
	}
	return ip
}

func (s *Server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.m.Ping(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "db ping failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) Update(ctx context.Context, in *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if in == nil || in.Metric == nil {
		return nil, status.Error(codes.InvalidArgument, "empty metric")
	}
	m := toModel(in.Metric)
	if err := s.m.UpdateMetricJSON(m); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "update failed: %v", err)
	}
	return &pb.UpdateResponse{Ok: true}, nil
}

func (s *Server) Updates(ctx context.Context, in *pb.UpdateBatchRequest) (*pb.UpdateBatchResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	list := make([]*model.Metrics, 0, len(in.Metrics))
	for _, pm := range in.Metrics {
		list = append(list, toModel(pm))
	}
	if err := s.m.UpdateMetricsBatch(list); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "batch failed: %v", err)
	}
	return &pb.UpdateBatchResponse{Ok: true}, nil
}

func (s *Server) Value(ctx context.Context, in *pb.GetValueRequest) (*pb.GetValueResponse, error) {
	if in == nil || in.Id == "" || in.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "id/type required")
	}
	m := &model.Metrics{ID: in.Id, MType: in.Type}
	if err := s.m.GetMetricJSON(m); err != nil {
		return &pb.GetValueResponse{Found: false}, nil
	}
	return &pb.GetValueResponse{
		Found:  true,
		Metric: toProto(m),
	}, nil
}

func toModel(pm *pb.Metric) *model.Metrics {
	m := &model.Metrics{ID: pm.Id, MType: pm.Type}
	if pm.Type == "gauge" {
		v := pm.Value
		m.Value = &v
	} else {
		d := pm.Delta
		m.Delta = &d
	}
	return m
}

func toProto(m *model.Metrics) *pb.Metric {
	pm := &pb.Metric{Id: m.ID, Type: m.MType}
	if m.MType == "gauge" && m.Value != nil {
		pm.Value = *m.Value
	}
	if m.MType == "counter" && m.Delta != nil {
		pm.Delta = *m.Delta
	}
	return pm
}

func BuildServer(trusted *net.IPNet, maxRecvMB, maxSendMB int) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxRecvMB * 1024 * 1024),
		grpc.MaxSendMsgSize(maxSendMB * 1024 * 1024),
		grpc.ChainUnaryInterceptor(trustedUnary(trusted)),
	}
	return grpc.NewServer(opts...)
}
