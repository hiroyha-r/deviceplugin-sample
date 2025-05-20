package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	resourceName = "example.com/hello-device"
	socket       = pluginapi.DevicePluginPath + "hello.sock"
)

type DevicePlugin struct {
	devices   []*pluginapi.Device
	server    *grpc.Server
	startedAt time.Time
}

func NewDevicePlugin() *DevicePlugin {
	devices := []*pluginapi.Device{
		{ID: "hello-device-1", Health: pluginapi.Healthy},
	}
	return &DevicePlugin{
		devices:   devices,
		startedAt: time.Now(),
	}
}

func (dp *DevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (dp *DevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (dp *DevicePlugin) ListAndWatch(_ *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {

	for {
		var devices []*pluginapi.Device
		now := time.Now()
		if now.Sub(dp.startedAt) > 20*time.Minute {
			devices = dp.devices
			fmt.Printf("Sending devices: %v\n", devices)
		} else {
			devices = []*pluginapi.Device{}
			fmt.Printf("No devices available yet | now: %s | startedAt: %s\n", now.Format(time.RFC3339), dp.startedAt.Format(time.RFC3339))
		}
		if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: devices}); err != nil {
			return err
		}
		time.Sleep(time.Second * 10)
	}
}

func (dp *DevicePlugin) Allocate(_ context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := &pluginapi.AllocateResponse{}
	for range req.ContainerRequests {
		resp.ContainerResponses = append(resp.ContainerResponses, &pluginapi.ContainerAllocateResponse{})
	}
	return resp, nil
}

func (dp *DevicePlugin) GetPreferredAllocation(_ context.Context, req *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	resp := &pluginapi.PreferredAllocationResponse{}
	for _, cr := range req.ContainerRequests {
		resp.ContainerResponses = append(resp.ContainerResponses, &pluginapi.ContainerPreferredAllocationResponse{
			DeviceIDs: cr.AvailableDeviceIDs[:cr.AllocationSize],
		})
	}
	return resp, nil
}

func (dp *DevicePlugin) Serve() error {
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	dp.server = grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(dp.server, dp)
	go dp.server.Serve(lis)
	return nil
}

func RegisterWithKubelet() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient("unix://"+pluginapi.KubeletSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to kubelet: %v", err)
	}
	defer conn.Close()
	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     "hello.sock",
		ResourceName: resourceName,
	}
	_, err = client.Register(ctx, req)
	return err
}

func main() {
	dp := NewDevicePlugin()
	if err := dp.Serve(); err != nil {
		fmt.Printf("Failed to serve: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Device plugin started, listening on %s\n", socket)

	if err := RegisterWithKubelet(); err != nil {
		fmt.Printf("Failed to register with kubelet: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Registered with kubelet\n")
	select {}
}
