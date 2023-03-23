/*
Copyright 2023.

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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	testv1alpha1 "github.com/abhiPL07/testOwner-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// TestOwnerReconciler reconciles a TestOwner object
type TestOwnerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=test.github.com,resources=testowners,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=test.github.com,resources=testowners/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=test.github.com,resources=testowners/finalizers,verbs=update
//+kubebuilder:rbac:groups=test.github.com,resources=testdependents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=test.github.com,resources=testdependents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=test.github.com,resources=testdependents/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TestOwner object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *TestOwnerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Reconciler start")

	// TODO(user): your logic here
	// Get Testowner instances
	testOwner := &testv1alpha1.TestOwner{}
	err := r.Get(ctx, req.NamespacedName, testOwner)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("TestOwner object was deleted\n")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Error getting TestOwner objects\n")
		return ctrl.Result{}, err
	}

	// Check if dependentList is empty
	dependentList := &testv1alpha1.TestDependentList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForDependent(testOwner.Name)),
	}
	err = r.List(ctx, dependentList, listOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	dependentNames := getDependentNames(dependentList.Items)
	// If no test-dependent exists, create test-dependent
	if len(dependentNames) == 0 {
		dependent := r.makeDependent(testOwner)
		err := r.Create(ctx, dependent)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else {
		// Else update label
		dependentName := dependentNames[0]
		dependent := &testv1alpha1.TestDependent{}
		err := r.Get(ctx, types.NamespacedName{Name: dependentName, Namespace: testOwner.Namespace}, dependent)
		if err != nil {
			log.Error(err, "Error fetching instance\n")
			return ctrl.Result{}, err
		}
		if testOwner.Spec.OwnerField == dependent.Spec.OwnerField {
			return ctrl.Result{Requeue: true}, nil
		}
		res, err := r.updateOwnerField(ctx, testOwner, dependent)
		if err != nil {
			log.Error(err, "Error updating ownerField")
			return res, err
		}
		return res, nil

	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *TestOwnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testv1alpha1.TestOwner{}).
		Owns(&testv1alpha1.TestDependent{}).
		Complete(r)
}

func (r *TestOwnerReconciler) makeDependent(testOwner *testv1alpha1.TestOwner) *testv1alpha1.TestDependent {
	lbls := labelsForDependent(testOwner.Name)
	ownerField := testOwner.Spec.OwnerField
	testDependent := &testv1alpha1.TestDependent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dependent",
			Namespace: testOwner.Namespace,
			Labels:    lbls,
		},
		Spec: testv1alpha1.TestDependentSpec{
			OwnerField: ownerField,
		},
	}
	controllerutil.SetControllerReference(testOwner, testDependent, r.Scheme)
	return testDependent
}

func labelsForDependent(name string) map[string]string {
	return map[string]string{"owner-name": name}
}

func (r *TestOwnerReconciler) updateOwnerField(ctx context.Context, testOwner *testv1alpha1.TestOwner, testDependent *testv1alpha1.TestDependent) (ctrl.Result, error) {
	ownerField := testOwner.Spec.OwnerField
	dependentField := testDependent.Spec.OwnerField
	if dependentField != ownerField {
		testDependent.Spec.OwnerField = ownerField
		if err := r.Update(ctx, testDependent); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{Requeue: true}, nil
}

func getDependentNames(dependents []testv1alpha1.TestDependent) []string {
	var dependentNames []string
	for _, dependent := range dependents {
		dependentNames = append(dependentNames, dependent.Name)
	}
	return dependentNames
}
