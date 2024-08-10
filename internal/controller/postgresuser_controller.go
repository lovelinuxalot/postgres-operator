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
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/lovelinuxalot/postgres-operator/api/v1alpha1"
	"github.com/lovelinuxalot/postgres-operator/internal/postgres"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Define finalizer string
var postgresUserFinalizer = "finalizer.postgresuser.pandarocks.de"

// PostgresUserReconciler reconciles a PostgresUser object
type PostgresUserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.pandarocks.de,resources=postgresusers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
// Reconcile is part of the main kubernetes reconciliation loop
func (r *PostgresUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("postgresuser", req.NamespacedName)

	// Fetch the PostgresUser instance
	var postgresUser v1alpha1.PostgresUser
	if err := r.Get(ctx, req.NamespacedName, &postgresUser); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Resource not found, probably deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the referenced PostgresDatabase
	var postgresDB v1alpha1.PostgresDatabase
	if err := r.Get(ctx, client.ObjectKey{Name: postgresUser.Spec.DatabaseRef.Name, Namespace: postgresUser.Namespace}, &postgresDB); err != nil {
		log.Error(err, "Failed to fetch referenced PostgresDatabase")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !postgresUser.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&postgresUser, postgresUserFinalizer) {
			// Only drop the user if dropOnDelete is true
			if postgresUser.Spec.DropOnDelete {
				// Get PostgreSQL credentials from the PostgresDatabase secret
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

				// Determine the username to use
				username := postgresUser.Spec.Username
				if username == "" {
					username = postgresUser.Name
				}
				// Drop the user if it exists
				if err := postgres.DropUser(db, username); err != nil {
					log.Error(err, "Failed to drop user")
					return ctrl.Result{}, err
				}
				log.Info("User dropped successfully", "username", username)
			}

			// Remove finalizer so the CR can be deleted
			controllerutil.RemoveFinalizer(&postgresUser, postgresUserFinalizer)
			if err := r.Update(ctx, &postgresUser); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the object is being deleted
		return ctrl.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&postgresUser, postgresUserFinalizer) {
		controllerutil.AddFinalizer(&postgresUser, postgresUserFinalizer)
		if err := r.Update(ctx, &postgresUser); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Determine the username to use
	username := postgresUser.Spec.Username
	if username == "" {
		username = postgresUser.Name
	}

	// Generate a password if it's not provided
	password := postgresUser.Spec.Password
	if password == "" && postgresUser.Spec.PasswordSpec != nil {
		password = postgres.GeneratePassword(postgresUser.Spec.PasswordSpec)
	}

	// Get PostgreSQL credentials from the PostgresDatabase secret
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

	// Create the user and assign them to the specified database
	if err := postgres.CreateUser(db, username, password, postgresUser.Spec.DatabaseRef.Name, postgresUser.Spec.RoleType); err != nil {
		log.Error(err, "Failed to create user")
		return ctrl.Result{}, err
	}

	// Create or update the secret with the user credentials
	if err := r.createOrUpdateSecret(ctx, &postgresUser, username, password, creds.Host, creds.Database); err != nil {
		log.Error(err, "Failed to create or update secret")
		return ctrl.Result{}, err
	}

	// Update status to reflect that the user was successfully created
	postgresUser.Status.Created = true
	postgresUser.Status.Message = "User successfully created"
	postgresUser.Status.SecretName = postgresUser.Spec.SecretName
	postgresUser.Status.SecretNamespace = postgresUser.ObjectMeta.Namespace

	if err := r.Status().Update(ctx, &postgresUser); err != nil {
		log.Error(err, "Failed to update PostgresUser status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PostgresUser{}).
		Complete(r)
}

// createOrUpdateSecret creates or updates a secret with the user credentials
func (r *PostgresUserReconciler) createOrUpdateSecret(ctx context.Context, postgresUser *v1alpha1.PostgresUser, username, password, host, database string) error {
	//Determine the name for the secret to be created
	secretName := postgresUser.Spec.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-postgres-user-credentials", postgresUser.Name)
	}

	// Check if secret exists
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: postgresUser.Namespace}, secret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	secretData := postgres.SecretData{
		Username: username,
		Password: password,
		Host:     host,
		Database: database,
	}

	var secretMap map[string]string
	if postgresUser.Spec.SecretTemplate != nil && postgresUser.Spec.SecretTemplate.TemplateData != "" {
		secretMap, err = postgres.ApplyTemplate(postgresUser.Spec.SecretTemplate.TemplateData, secretData)
		if err != nil {
			return err
		}
	} else {
		// Default secret data
		secretMap = map[string]string{
			"username": username,
			"password": password,
			"host":     host,
			"database": database,
		}
	}

	if errors.IsNotFound(err) {
		// Secret does not exist, create it
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: postgresUser.Namespace,
			},
			StringData: secretMap,
		}
		// Apply SecretTemplate if provided
		if postgresUser.Spec.SecretTemplate != nil {
			secret.ObjectMeta.Labels = postgresUser.Spec.SecretTemplate.Labels
			secret.ObjectMeta.Annotations = postgresUser.Spec.SecretTemplate.Annotations
		}
		if err := r.Create(ctx, secret); err != nil {
			return err
		}
	} else {
		// Secret exists, update it
		secret.StringData = secretMap
		if err := r.Update(ctx, secret); err != nil {
			return err
		}
	}

	return nil
}
