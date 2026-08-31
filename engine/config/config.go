package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Backend                string
	DataDir                string
	SchemaDir              string
	AuthDatabaseFile       string
	CharactersDatabaseFile string
	WorldDatabaseFile      string
	LoginDatabaseInfo      string
	WorldDatabaseInfo      string
	CharacterDatabaseInfo  string
	RealmServerPort        int
	WorldServerPort        int
	RealmID                uint32
	LogsDir                string
	LuaEnabled             bool
	LuaScriptPath          string
}

func Default() Config {
	return Config{Backend: "sqlite", DataDir: ".", SchemaDir: "sql", AuthDatabaseFile: "auth.db", CharactersDatabaseFile: "characters.db", WorldDatabaseFile: "world.db", RealmServerPort: 3724, WorldServerPort: 8085, RealmID: 1, LogsDir: "logs", LuaEnabled: true, LuaScriptPath: "lua_scripts"}
}

func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		return c, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	line := 0
	for s.Scan() {
		line++
		key, value, ok := split(s.Text())
		if !ok {
			continue
		}
		if err := c.set(key, value); err != nil {
			return c, fmt.Errorf("%s:%d: %w", path, line, err)
		}
	}
	if err := s.Err(); err != nil {
		return c, err
	}
	return c, nil
}

func (c *Config) ApplyEnv() {
	values := map[string]string{"MORENO_TRINITYGO_BACKEND": "Database.Backend", "MORENO_TRINITYGO_DATA_DIR": "DataDir", "MORENO_TRINITYGO_SCHEMA_DIR": "SchemaDir", "MORENO_TRINITYGO_AUTH_DB": "AuthDatabaseFile", "MORENO_TRINITYGO_CHARACTERS_DB": "CharactersDatabaseFile", "MORENO_TRINITYGO_WORLD_DB": "WorldDatabaseFile", "MORENO_TRINITYGO_LOGIN_DATABASE": "LoginDatabaseInfo", "MORENO_TRINITYGO_WORLD_DATABASE": "WorldDatabaseInfo", "MORENO_TRINITYGO_CHARACTER_DATABASE": "CharacterDatabaseInfo", "MORENO_TRINITYGO_REALM_PORT": "RealmServerPort", "MORENO_TRINITYGO_WORLD_PORT": "WorldServerPort", "MORENO_TRINITYGO_REALM_ID": "RealmID", "MORENO_TRINITYGO_LOGS_DIR": "LogsDir", "MORENO_TRINITYGO_LUA_ENABLED": "Eluna.Enabled", "MORENO_TRINITYGO_LUA_PATH": "Eluna.ScriptPath"}
	for env, key := range values {
		if value, ok := os.LookupEnv(env); ok {
			_ = c.set(key, value)
		}
	}
}

func (c *Config) Set(key, value string) error { return c.set(key, value) }

func (c Config) DatabasePath(name string) string {
	return filepath.Clean(filepath.Join(c.DataDir, name))
}

func split(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return "", "", false
	}
	i := strings.IndexByte(line, '=')
	if i < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:i])
	value := strings.TrimSpace(line[i+1:])
	if n := strings.Index(value, " #"); n >= 0 {
		value = strings.TrimSpace(value[:n])
	}
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

func (c *Config) set(key, value string) error {
	switch key {
	case "Database.Backend":
		c.Backend = strings.ToLower(value)
	case "DataDir":
		c.DataDir = value
	case "SchemaDir":
		c.SchemaDir = value
	case "AuthDatabaseFile":
		c.AuthDatabaseFile = value
	case "CharactersDatabaseFile":
		c.CharactersDatabaseFile = value
	case "WorldDatabaseFile":
		c.WorldDatabaseFile = value
	case "LoginDatabaseInfo":
		c.LoginDatabaseInfo = value
	case "WorldDatabaseInfo":
		c.WorldDatabaseInfo = value
	case "CharacterDatabaseInfo":
		c.CharacterDatabaseInfo = value
	case "RealmServerPort":
		return setInt(&c.RealmServerPort, key, value)
	case "WorldServerPort":
		return setInt(&c.WorldServerPort, key, value)
	case "RealmID":
		return setUint32(&c.RealmID, key, value)
	case "LogsDir":
		c.LogsDir = value
	case "Eluna.Enabled":
		return setBool(&c.LuaEnabled, key, value)
	case "Eluna.ScriptPath":
		c.LuaScriptPath = value
	}
	return nil
}

func setInt(dst *int, key, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer: %w", key, err)
	}
	*dst = n
	return nil
}

func setBool(dst *bool, key, value string) error {
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be boolean: %w", key, err)
	}
	*dst = b
	return nil
}

func setUint32(dst *uint32, key, value string) error {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return fmt.Errorf("%s must be an unsigned integer: %w", key, err)
	}
	*dst = uint32(n)
	return nil
}

func (c Config) Validate() error {
	if c.Backend != "sqlite" && c.Backend != "mysql" && c.Backend != "mariadb" {
		return errors.New("Database.Backend must be sqlite, mysql, or mariadb")
	}
	if c.RealmServerPort < 1 || c.RealmServerPort > 65535 || c.WorldServerPort < 1 || c.WorldServerPort > 65535 {
		return errors.New("server ports must be between 1 and 65535")
	}
	return nil
}
