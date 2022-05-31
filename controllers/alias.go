/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.bj
*/

// Package controllers provides access to reconcilers implemented in internal/controllers.
package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	ccontrollers "github.com/raminenia/cluster-api-provider-containerd/internal/controllers"
	"sigs.k8s.io/cluster-api/test/infrastructure/container"
)

// Following types provides access to reconcilers implemented in internal/controllers, thus
// allowing users to provide a single binary "batteries included" with Cluster API and providers of choice.

// ContainerdMachineReconciler reconciles a ContainerdMachine object.
type ContainerdMachineReconciler struct {
	Client           client.Client
	ContainerRuntime container.Runtime
}

// SetupWithManager sets up the reconciler with the Manager.
func (r *ContainerdMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return (&ccontrollers.ContainerdMachineReconciler{
		Client:           r.Client,
		ContainerRuntime: r.ContainerRuntime,
	}).SetupWithManager(ctx, mgr, options)
}

// ContainerdClusterReconciler reconciles a DockerMachine object.
type ContainerdClusterReconciler struct {
	Client           client.Client
	ContainerRuntime container.Runtime
}

// SetupWithManager sets up the reconciler with the Manager.
func (r *ContainerdClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return (&ccontrollers.ContainerdClusterReconciler{
		Client:           r.Client,
		ContainerRuntime: r.ContainerRuntime,
	}).SetupWithManager(ctx, mgr, options)
}
