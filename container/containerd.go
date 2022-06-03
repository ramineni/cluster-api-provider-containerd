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
	//"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/namespaces"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/containerd/nerdctl/pkg/labels/k8slabels"
	"github.com/docker/docker/errdefs"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/cluster-api/test/infrastructure/container"
)

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
	containers, err := c.client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	var result []container.Container
	for _, c := range containers {
		info, err := c.Info(ctx, containerd.WithoutRefreshedMetadata)
		if err != nil {
			if errdefs.IsNotFound(err) {
				logrus.Warn(err)
				continue
			}
			return nil, err
		}
		imageName := info.Image
		cStatus := formatter.ContainerStatus(ctx, c)

		p := container.Container{
			Name:   getPrintableContainerName(info.Labels),
			Image:  imageName,
			Status: cStatus,
		}
		result = append(result, p)
	}
	return result, nil
}

func (c *containerdRuntime) ContainerDebugInfo(ctx context.Context, containerName string, w io.Writer) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) DeleteContainer(ctx context.Context, containerName string) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) KillContainer(ctx context.Context, containerName, signal string) error {
	return fmt.Errorf("not implemented")
}

func getPrintableContainerName(containerLabels map[string]string) string {
	if name, ok := containerLabels[labels.Name]; ok {
		return name
	}

	if ns, ok := containerLabels[k8slabels.PodNamespace]; ok {
		if podName, ok := containerLabels[k8slabels.PodName]; ok {
			if containerName, ok := containerLabels[k8slabels.ContainerName]; ok {
				// Container
				return fmt.Sprintf("k8s://%s/%s/%s", ns, podName, containerName)
			} else {
				// Pod sandbox
				return fmt.Sprintf("k8s://%s/%s", ns, podName)
			}
		}
	}
	return ""
}
