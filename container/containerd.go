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
	//"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/cap"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/nerdctl/pkg/idgen"
	"github.com/containerd/nerdctl/pkg/idutil/containerwalker"
	"github.com/containerd/nerdctl/pkg/strutil"
	"github.com/containerd/nerdctl/pkg/taskutil"
	"github.com/opencontainers/runtime-spec/specs-go"

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
	arg := append([]string{command}, args...)
	walker := &containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			if found.MatchCount > 1 {
				return fmt.Errorf("ambiguous ID %q", found.Req)
			}
			return execActionWithContainer(ctx, config, arg, found.Container, c.client)
		},
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such container %s", containerName)
	}
	return nil
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

func execActionWithContainer(ctx context.Context, config *container.ExecContainerInput, args []string, container containerd.Container, client *containerd.Client) error {
	flagI := config.InputBuffer != nil

	pspec, err := generateExecProcessSpec(ctx, config, args, container, client)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	var (
		ioCreator cio.Creator
		in        io.Reader
		stdinC    = &taskutil.StdinCloser{
			Stdin: os.Stdin,
		}
	)

	if flagI {
		in = stdinC
	}
	cioOpts := []cio.Opt{cio.WithStreams(in, os.Stdout, os.Stderr)}
	ioCreator = cio.NewCreator(cioOpts...)

	execID := "exec-" + idgen.GenerateID()
	process, err := task.Exec(ctx, execID, pspec, ioCreator)
	if err != nil {
		return err
	}
	stdinC.Closer = func() {
		process.CloseIO(ctx, containerd.WithStdinCloser)
	}
	defer process.Delete(ctx)

	statusC, err := process.Wait(ctx)
	if err != nil {
		return err
	}

	sigc := commands.ForwardAllSignals(ctx, process)
	defer commands.StopCatch(sigc)

	if err := process.Start(ctx); err != nil {
		return err
	}

	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("exec failed with exit code %d", code)
	}
	return nil
}

func generateExecProcessSpec(ctx context.Context, config *container.ExecContainerInput, args []string, container containerd.Container, client *containerd.Client) (*specs.Process, error) {
	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, err
	}

	pspec := spec.Process
	pspec.Args = args

	env := config.EnvironmentVars
	if err != nil {
		return nil, err
	}
	for _, e := range strutil.DedupeStrSlice(env) {
		pspec.Env = append(pspec.Env, e)
	}

	privileged := true
	if privileged {
		err = setExecCapabilities(pspec)
		if err != nil {
			return nil, err
		}
	}

	return pspec, nil
}

func setExecCapabilities(pspec *specs.Process) error {
	if pspec.Capabilities == nil {
		pspec.Capabilities = &specs.LinuxCapabilities{}
	}
	allCaps, err := cap.Current()
	if err != nil {
		return err
	}
	pspec.Capabilities.Bounding = allCaps
	pspec.Capabilities.Permitted = pspec.Capabilities.Bounding
	pspec.Capabilities.Inheritable = pspec.Capabilities.Bounding
	pspec.Capabilities.Effective = pspec.Capabilities.Bounding

	// https://github.com/moby/moby/pull/36466/files
	// > `docker exec --privileged` does not currently disable AppArmor
	// > profiles. Privileged configuration of the container is inherited
	return nil
}
