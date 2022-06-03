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
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/nerdctl/pkg/platformutil"
	"github.com/containerd/nerdctl/pkg/referenceutil"

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
	var saveOpts = []archive.ExportOpt{}

	tar, err := os.Create(dest) //nolint:gosec // No security issue: dest is safe.
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %v", dest, err)
	}
	defer tar.Close()

	platform := []string{"amd64"}
	platMC, err := platformutil.NewMatchComparer(false, platform)
	if err != nil {
		return err
	}

	saveOpts = append(saveOpts, archive.WithPlatform(platMC))

	imageStore := c.client.ImageService()
	named, err := referenceutil.ParseAny(image)
	if err != nil {
		return err
	}

	saveOpts = append(saveOpts, archive.WithImage(imageStore, named.String()))
	return c.client.Export(ctx, tar, saveOpts...)
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

func (c *containerdRuntime) DeleteContainer(ctx context.Context, containerName string) error {
	return fmt.Errorf("not implemented")
}

func (c *containerdRuntime) KillContainer(ctx context.Context, containerName, signal string) error {
	return fmt.Errorf("not implemented")
}
