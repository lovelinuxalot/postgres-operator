package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Credentials struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
	UriArgs  string
}

// GetCredentials fetches the PostgreSQL credentials from a secret
func GetCredentials(ctx context.Context, kubeClient client.Client, secretName, secretNamespace string) (Credentials, error) {
	var secret corev1.Secret
	if err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: secretNamespace,
		Name:      secretName,
	}, &secret); err != nil {
		return Credentials{}, err
	}

	return Credentials{
		Host:     string(secret.Data["POSTGRES_HOST"]),
		Port:     string(secret.Data["POSTGRES_PORT"]),
		Username: string(secret.Data["POSTGRES_USER"]),
		Password: string(secret.Data["POSTGRES_PASSWORD"]),
		Database: string(secret.Data["POSTGRES_DATABASE"]),
		UriArgs:  string(secret.Data["POSTGRES_URI_ARGS"]),
	}, nil
}

// Connect establishes a connection to the PostgreSQL server
func Connect(creds Credentials) (*sql.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s",
		creds.Username, creds.Password, creds.Host, creds.Port, creds.Database, creds.UriArgs)
	return sql.Open("postgres", dsn)
}
