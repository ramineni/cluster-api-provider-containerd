/*
Copyright 2022 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package container provides an interface for interacting with containerd
package container

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	refdocker "github.com/containerd/containerd/reference/docker"
	sysignal "github.com/moby/sys/signal"

	"sigs.k8s.io/cluster-api/test/infrastructure/container"
)

const defaultSignal = "SIGTERM"

type containerdRuntime struct {
	client    *containerd.Client
	namespace string
}

func NewContainerdClient(socketPath string, namespace string) (container.Runtime, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return &containerdRuntime{}, fmt.Errorf("failed to create containerd client")
	}

	return &containerdRuntime{client: client, namespace: namespace}, nil
}

func (c *containerdRuntime) SaveContainerImage(ctx context.Context, image, dest string) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) PullContainerImageIfNotExists(ctx context.Context, image string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	ref, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %v", err)
	}

	images, err := c.client.ListImages(ctx, fmt.Sprintf("name==%s", ref.String()))
	if err != nil {
		return fmt.Errorf("error listing images: %v", err)
	}

	// image already exists
	if len(images) > 0 {
		return nil
	}

	if _, err := c.client.Pull(ctx, image); err != nil {
		return fmt.Errorf("error pulling image: %v", err)
	}

	return nil
}

func (c *containerdRuntime) GetHostPort(ctx context.Context, containerName, portAndProtocol string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (c *containerdRuntime) GetContainerIPs(ctx context.Context, containerName string) (string, string, error) {
	return "", "", fmt.Errorf("not implemented")
}

func (c *containerdRuntime) ExecContainer(ctx context.Context, containerName string, config *container.ExecContainerInput, command string, args ...string) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) RunContainer(ctx context.Context, runConfig *container.RunContainerInput, output io.Writer) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) ListContainers(ctx context.Context, filters container.FilterBuilder) ([]container.Container, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *containerdRuntime) ContainerDebugInfo(ctx context.Context, containerName string, w io.Writer) error {
	return fmt.Errorf("not implemented")
}

// DeleteContainer will remove a container.
func (c *containerdRuntime) DeleteContainer(ctx context.Context, containerName string) error {
	deleteOpts := []containerd.DeleteOpts{}
	deleteOpts = append(deleteOpts, containerd.WithSnapshotCleanup) // delete volumes
	container, err := c.client.LoadContainer(ctx, containerName)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, cio.Load)
	if err != nil {
		return container.Delete(ctx, deleteOpts...)
	}
	status, err := task.Status(ctx)
	if err != nil {
		return err
	}
	if status.Status == containerd.Stopped || status.Status == containerd.Created {
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		return container.Delete(ctx, deleteOpts...)
	}
	return fmt.Errorf("cannot delete a non stopped container: %v", status)
}

// KillContainer will kill all running tasks in a container with the specified signal.
func (c *containerdRuntime) KillContainer(ctx context.Context, containerName, signal string) error {
	sig, err := sysignal.ParseSignal(defaultSignal)
	if err != nil {
		return err
	}
	opts := []containerd.KillOpts{}
	opts = append(opts, containerd.WithKillAll) // send signal to all processes inside the container
	container, err := c.client.LoadContainer(ctx, containerName)
	if err != nil {
		return err
	}
	if signal != "" {
		sig, err = sysignal.ParseSignal(signal)
		if err != nil {
			return err
		}
	} else {
		sig, err = containerd.GetStopSignal(ctx, container, sig)
		if err != nil {
			return err
		}
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	// err = tasks.RemoveCniNetworkIfExist(ctx, container)
	// if err != nil {
	// 	return err
	// }
	return task.Kill(ctx, sig, opts...)
}
