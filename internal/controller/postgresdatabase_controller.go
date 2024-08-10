/*
Copyright 2024.

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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/lovelinuxalot/postgres-operator/api/v1alpha1"
	"github.com/lovelinuxalot/postgres-operator/internal/postgres"
)

// Define finalizer string
var postgresDatabaseFinalizer = "finalizer.postgresdatabase.pandarocks.de"

// PostgresDatabaseReconciler reconciles a PostgresDatabase object
type PostgresDatabaseReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresdatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresdatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *PostgresDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("postgresdatabase", req.NamespacedName)

	// Check if the resource exists
	var postgresDB v1alpha1.PostgresDatabase
	if err := r.Get(ctx, req.NamespacedName, &postgresDB); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Resource not found, probably deleted or not existing")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !postgresDB.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&postgresDB, postgresDatabaseFinalizer) {
			//  Only drop the database if dropOnDelete is true
			if postgresDB.Spec.DropOnDelete {
				// Get PostgreSQL credentials from the secret
				creds, err := postgres.GetCredentials(ctx, r.Client, postgresDB.Spec.ServerSecretRef.Name, postgresDB.Spec.ServerSecretRef.Namespace)
				if err != nil {
					log.Error(err, "Unable to fetch PostgreSQL credentials from secret")
					return ctrl.Result{}, err
				}

				// Establish a connection to the PostgreSQL server
				db, err := postgres.Connect(creds)
				if err != nil {
					log.Error(err, "Unable to connect to PostgreSQL server")
					return ctrl.Result{}, err
				}
				defer db.Close()

				// Drop the database if it exists
				if err := postgres.DropDatabase(db, req.Name); err != nil {
					log.Error(err, "Failed to drop database")
					return ctrl.Result{}, err
				}
				log.Info("Database dropped successfully", "database", req.Name)
			}

			// Remove finalizer so the CR can be deleted
			controllerutil.RemoveFinalizer(&postgresDB, postgresDatabaseFinalizer)
			if err := r.Update(ctx, &postgresDB); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the object is being deleted
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(&postgresDB, postgresDatabaseFinalizer) {
		controllerutil.AddFinalizer(&postgresDB, postgresDatabaseFinalizer)
		if err := r.Update(ctx, &postgresDB); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get PostgreSQL credentials from the secret
	creds, err := postgres.GetCredentials(ctx, r.Client, postgresDB.Spec.ServerSecretRef.Name, postgresDB.Spec.ServerSecretRef.Namespace)
	if err != nil {
		log.Error(err, "Unable to fetch PostgreSQL credentials from secret")
		return ctrl.Result{}, err
	}

	// Establish a connection to the PostgreSQL server
	db, err := postgres.Connect(creds)
	if err != nil {
		log.Error(err, "Unable to connect to PostgreSQL server")
		return ctrl.Result{}, err
	}
	log.Info("Postgres connection established")
	defer db.Close()

	databaseName := req.Name

	// Create the database
	if err := postgres.CreateDatabase(db, databaseName); err != nil {
		log.Error(err, "Failed to create database")
		return ctrl.Result{}, err
	}

	// Update status
	postgresDB.Status.Created = true
	postgresDB.Status.Message = "Database successfully created"

	if err := r.Status().Update(ctx, &postgresDB); err != nil {
		log.Error(err, "Failed to update PostgresDatabase status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PostgresDatabase{}).
		Complete(r)
}
