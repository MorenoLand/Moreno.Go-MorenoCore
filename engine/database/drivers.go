package database

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

func connection(backend, file, info string, c config.Config) (string, string, Backend, error) {
	switch strings.ToLower(backend) {
	case "sqlite":
		if file == "" {
			return "", "", "", errors.New("SQLite database file is empty")
		}
		return "sqlite", filepath.Clean(filepath.Join(c.DataDir, file)), BackendSQLite, nil
	case "mysql", "mariadb":
		dsn, err := mysqlDSN(info)
		if err != nil {
			return "", "", "", err
		}
		kind := BackendMySQL
		if strings.EqualFold(backend, "mariadb") {
			kind = BackendMariaDB
		}
		return "mysql", dsn, kind, nil
	default:
		return "", "", "", fmt.Errorf("unsupported backend %q", backend)
	}
}

func mysqlDSN(info string) (string, error) {
	parts := strings.Split(info, ";")
	if len(parts) != 5 {
		return "", errors.New("database info must be host;port;user;password;database")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("database info contains an invalid port")
	}
	host := parts[0]
	if host == "" || parts[2] == "" || parts[4] == "" {
		return "", errors.New("database info requires host, user, and database")
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&multiStatements=true", parts[2], parts[3], net.JoinHostPort(host, strconv.Itoa(port)), parts[4]), nil
}
