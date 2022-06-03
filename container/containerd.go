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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	refdocker "github.com/containerd/containerd/reference/docker"
	gocni "github.com/containerd/go-cni"
	"github.com/containerd/nerdctl/pkg/containerinspector"
	"github.com/containerd/nerdctl/pkg/idutil/containerwalker"
	"github.com/containerd/nerdctl/pkg/inspecttypes/dockercompat"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/containerd/nerdctl/pkg/logging/jsonfile"
	"github.com/docker/cli/templates"
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
	argPort := -1
	argProto := ""
	portProto := portAndProtocol
	var err error

	if portProto != "" {
		splitBySlash := strings.Split(portProto, "/")
		argPort, err = strconv.Atoi(splitBySlash[0])
		if err != nil {
			return "", err
		}
		if argPort <= 0 {
			return "", fmt.Errorf("unexpected port %d", argPort)
		}
		switch len(splitBySlash) {
		case 1:
			argProto = "tcp"
		case 2:
			argProto = strings.ToLower(splitBySlash[1])
		default:
			return "", fmt.Errorf("failed to parse %q", portProto)
		}
	}

	var port string
	walker := &containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			if found.MatchCount > 1 {
				return fmt.Errorf("ambiguous ID %q", found.Req)
			}
			port, err = printPort(ctx, found.Req, found.Container, argPort, argProto)
			if err != nil {
				return err
			}
			return nil
		},
	}

	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return "", err
	} else if n == 0 {
		return "", fmt.Errorf("no such container %s", containerName)
	}
	return port, nil
}

func (c *containerdRuntime) GetContainerIPs(ctx context.Context, containerName string) (string, string, error) {
	f := &containerInspector{}
	walker := containerwalker.ContainerWalker{
		Client:  c.client,
		OnFound: f.Handler,
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return "", "", err
	} else if n == 0 {
		return "", "", fmt.Errorf("no such object %s", containerName)
	}

	format := "{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}"
	ip, err := formatSlice(f.entries, format)
	if err != nil {
		return "", "", err
	}

	format = "{{range.NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}"
	ipv6, err := formatSlice(f.entries, format)
	if err != nil {
		return "", "", err
	}

	return ip, ipv6, nil
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

// ContainerDebugInfo gets the container metadata and logs.
// Currently, only containers created with `nerdctl run -d` are supported for log collection.
func (c *containerdRuntime) ContainerDebugInfo(ctx context.Context, containerName string, w io.Writer) error {
	f := &containerInspector{}
	walker := containerwalker.ContainerWalker{
		Client:  c.client,
		OnFound: f.Handler,
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such object %s", containerName)
	}

	containerInfo, err := json.MarshalIndent(f.entries, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "Inspected the container:")
	fmt.Fprintf(w, "%+v\n", string(containerInfo))

	// "1935db9" is from `$(echo -n "/run/containerd/containerd.sock" | sha256sum | cut -c1-8)``
	// on Windows it will return "%PROGRAMFILES%/nerdctl/1935db59"
	dataStore := "/var/lib/nerdctl/1935db59"
	ns := "default"

	walker = containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			logJSONFilePath := jsonfile.Path(dataStore, ns, found.Container.ID())
			if _, err := os.Stat(logJSONFilePath); err != nil {
				return fmt.Errorf("failed to open %q, container is not created with `nerdctl run -d`?: %w", logJSONFilePath, err)
			}
			var reader io.Reader
			//chan for non-follow tail to check the logsEOF
			logsEOFChan := make(chan struct{})
			f, err := os.Open(logJSONFilePath)
			if err != nil {
				return err
			}
			defer f.Close()
			reader = f
			go func() {
				<-logsEOFChan
			}()

			fmt.Fprintln(w, "Got logs from the container:")
			return jsonfile.Decode(w, w, reader, false, "", "", logsEOFChan)
		},
	}
	n, err = walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such container %s", containerName)
	}
	return nil
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

func printPort(ctx context.Context, containerName string, container containerd.Container, argPort int, argProto string) (string, error) {
	l, err := container.Labels(ctx)
	if err != nil {
		return "", err
	}
	portsJSON := l[labels.Ports]
	if portsJSON == "" {
		return "", nil
	}
	var ports []gocni.PortMapping
	if err := json.Unmarshal([]byte(portsJSON), &ports); err != nil {
		return "", err
	}
	// Loop through the ports and return the first HostPort.
	for _, p := range ports {
		if p.ContainerPort == int32(argPort) && strings.ToLower(p.Protocol) == argProto {
			return strconv.Itoa(int(p.HostPort)), nil
		}
	}
	return "", fmt.Errorf("no host port found for load balancer %q", containerName)
}

type containerInspector struct {
	entries []interface{}
}

func (x *containerInspector) Handler(ctx context.Context, found containerwalker.Found) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	n, err := containerinspector.Inspect(ctx, found.Container)
	if err != nil {
		return err
	}

	d, err := dockercompat.ContainerFromNative(n)
	if err != nil {
		return err
	}
	x.entries = append(x.entries, d)
	return nil
}

func formatSlice(x []interface{}, format string) (string, error) {
	var tmpl *template.Template
	var err error
	tmpl, err = parseTemplate(format)
	if err != nil {
		return "", err
	}
	for _, f := range x {
		var b bytes.Buffer
		if err := tmpl.Execute(&b, f); err != nil {
			if _, ok := err.(template.ExecError); ok {
				// FallBack to Raw Format
				if err = tryRawFormat(&b, f, tmpl); err != nil {
					return "", err
				}
			}
		}
		return b.String(), nil
	}
	return "", nil
}

func tryRawFormat(b *bytes.Buffer, f interface{}, tmpl *template.Template) error {
	m, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		return err
	}

	var raw interface{}
	rdr := bytes.NewReader(m)
	dec := json.NewDecoder(rdr)
	dec.UseNumber()

	if rawErr := dec.Decode(&raw); rawErr != nil {
		return fmt.Errorf("unable to read inspect data: %v", rawErr)
	}

	tmplMissingKey := tmpl.Option("missingkey=error")
	if rawErr := tmplMissingKey.Execute(b, raw); rawErr != nil {
		return fmt.Errorf("Template parsing error: %v", rawErr)
	}

	return nil
}

// parseTemplate wraps github.com/docker/cli/templates.Parse() to allow `json` as an alias of `{{json .}}`.
// parseTemplate can be removed when https://github.com/docker/cli/pull/3355 gets merged and tagged (Docker 22.XX).
func parseTemplate(format string) (*template.Template, error) {
	aliases := map[string]string{
		"json": "{{json .}}",
	}
	if alias, ok := aliases[format]; ok {
		format = alias
	}
	return templates.Parse(format)
}
