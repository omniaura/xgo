package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type storagePolicy struct {
	CacheDir      string        `json:"cache_dir"`
	MaxBytes      int64         `json:"max_bytes"`
	MinFreeBytes  int64         `json:"min_free_bytes"`
	TempMaxAge    time.Duration `json:"temp_max_age"`
	CheckInterval time.Duration `json:"check_interval"`
}

var storageKeys = []string{"cache-dir", "cache-max-size", "cache-min-free", "temp-max-age", "disk-check-interval"}

func loadStoragePolicy(projectDir string, flags map[string]string) (storagePolicy, error) {
	v := viper.New()
	v.SetEnvPrefix("XGO")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	v.SetDefault("cache-max-size", "20GiB")
	v.SetDefault("cache-min-free", "10GiB")
	v.SetDefault("temp-max-age", "24h")
	v.SetDefault("disk-check-interval", "2s")
	if projectDir == "" {
		projectDir = "."
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return storagePolicy{}, err
	}
	configFile := flags["config"]
	if configFile == "" {
		configFile = os.Getenv("XGO_CONFIG")
	}
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return storagePolicy{}, err
		}
	} else {
		// Nearest project config wins; stop at the module boundary.
		for {
			candidate := filepath.Join(root, ".xgo.yaml")
			_, err := os.Stat(candidate)
			if err == nil {
				v.SetConfigFile(candidate)
				if err := v.ReadInConfig(); err != nil {
					return storagePolicy{}, err
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return storagePolicy{}, err
			}
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				break
			}
			root = parent
		}
	}
	allowed := map[string]bool{}
	for _, key := range storageKeys {
		allowed[key] = true
	}
	for _, key := range v.AllKeys() {
		if !allowed[key] {
			return storagePolicy{}, fmt.Errorf("unknown xgo configuration key %q", key)
		}
	}
	for _, key := range storageKeys {
		if val, ok := flags[key]; ok {
			v.Set(key, val)
		}
	}
	var p storagePolicy
	p.CacheDir = v.GetString("cache-dir")
	if p.CacheDir != "" && !filepath.IsAbs(p.CacheDir) {
		base := projectDir
		_, fromFlag := flags["cache-dir"]
		fromEnv := os.Getenv("XGO_CACHE_DIR") != ""
		if !fromFlag && !fromEnv && v.InConfig("cache-dir") {
			base = filepath.Dir(v.ConfigFileUsed())
		}
		p.CacheDir, err = filepath.Abs(filepath.Join(base, p.CacheDir))
		if err != nil {
			return p, err
		}
	}
	if p.MaxBytes, err = parseStorageBytes(v.GetString("cache-max-size")); err != nil {
		return p, fmt.Errorf("cache-max-size: %w", err)
	}
	if p.MinFreeBytes, err = parseStorageBytes(v.GetString("cache-min-free")); err != nil {
		return p, fmt.Errorf("cache-min-free: %w", err)
	}
	if p.TempMaxAge, err = time.ParseDuration(v.GetString("temp-max-age")); err != nil {
		return p, fmt.Errorf("temp-max-age: %w", err)
	}
	if p.CheckInterval, err = time.ParseDuration(v.GetString("disk-check-interval")); err != nil {
		return p, fmt.Errorf("disk-check-interval: %w", err)
	}
	if p.MaxBytes <= 0 || p.TempMaxAge < time.Minute || p.CheckInterval < 100*time.Millisecond || p.CheckInterval > time.Minute {
		return p, fmt.Errorf("cache-max-size must be positive, temp-max-age >= 1m, disk-check-interval between 100ms and 1m")
	}
	return p, nil
}

func parseStorageBytes(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	multipliers := []struct {
		suffix string
		n      int64
	}{{"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10}, {"GB", 1000000000}, {"MB", 1000000}, {"KB", 1000}, {"B", 1}}
	mult := int64(1)
	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			s = strings.TrimSpace(strings.TrimSuffix(s, m.suffix))
			mult = m.n
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || n > (1<<63-1)/mult {
		return 0, fmt.Errorf("invalid byte size %q (use e.g. 20GiB)", s)
	}
	return n * mult, nil
}
