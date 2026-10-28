package main

import (
	"fmt"
	"os"

	test_explorer "github.com/omniaura/xgo/cmd/xgo/test-explorer"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "xgo", Short: "Instrument Go builds, tests, and programs", Example: "  xgo test --cache-max-size 20GiB ./...\n  xgo run . program-arguments\n  xgo cache status", SilenceUsage: true, SilenceErrors: true}
	addStorageFlags(root)
	root.PersistentFlags().AddFlagSet(root.Flags())
	for _, name := range []string{"build", "test", "run", "exec", "setup"} {
		name := name
		c := &cobra.Command{Use: name + " [xgo flags] [go arguments]", Short: "Instrumented go " + name, DisableFlagParsing: true, RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return c.Help()
			}
			return superviseBuild(name, args)
		}}
		addStorageFlags(c)
		root.AddCommand(c)
	}
	root.AddCommand(&cobra.Command{Use: "__storage-worker", Hidden: true, DisableFlagParsing: true, RunE: func(c *cobra.Command, args []string) error {
		if !storageProcessGroups {
			return fmt.Errorf("runtime supervisor is supported on macOS/Linux")
		}
		if len(args) == 0 || os.Getenv("XGO_STORAGE_SESSION") == "" {
			return fmt.Errorf("internal storage worker requires a supervised session")
		}
		session := os.Getenv("XGO_STORAGE_SESSION")
		lease, err := lockCache(session, false, true)
		if err != nil {
			return err
		}
		defer lease.close()
		if _, err = recordStorageGroup(session, os.Getpid()); err != nil {
			return err
		}
		pool := os.Getenv("XGO_STORAGE_TEMP")
		if pool == "" {
			return fmt.Errorf("internal storage worker requires a managed temporary pool")
		}
		poolLease, err := lockCache(pool, false, true)
		if err != nil {
			return err
		}
		defer poolLease.close()
		return handleBuild(args[0], args[1:])
	}})
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print xgo version", Args: cobra.NoArgs, Run: func(c *cobra.Command, args []string) { fmt.Fprintln(c.OutOrStdout(), VERSION) }})
	revision := &cobra.Command{Use: "revision", Short: "Print xgo revision", Args: cobra.NoArgs, Run: func(c *cobra.Command, args []string) {
		core, _ := c.Flags().GetBool("core")
		if core {
			fmt.Fprintln(c.OutOrStdout(), getCoreRevision())
		} else {
			fmt.Fprintln(c.OutOrStdout(), getRevision())
		}
	}}
	revision.Flags().Bool("core", false, "Print instrumentation core revision")
	root.AddCommand(revision)
	root.AddCommand(&cobra.Command{Use: "upgrade", Short: "Upgrade xgo", DisableFlagParsing: true, RunE: func(c *cobra.Command, args []string) error { return handleUpgrade(args) }})
	root.AddCommand(&cobra.Command{Use: "tool", Short: "Invoke xgo tools", DisableFlagParsing: true, RunE: func(c *cobra.Command, args []string) error { return handleTool(args) }})
	root.AddCommand(&cobra.Command{Use: "test-explorer", Aliases: []string{"explorer", "e"}, Short: "Open the test explorer", DisableFlagParsing: true, RunE: func(c *cobra.Command, args []string) error {
		return test_explorer.Main(args, &test_explorer.Options{DefaultGoCommand: "xgo"})
	}})
	root.AddCommand(&cobra.Command{Use: "shadow", Hidden: true, RunE: func(c *cobra.Command, args []string) error {
		return fmt.Errorf("shadow is deprecated, use `xgo setup` instead")
	}})
	cache := &cobra.Command{Use: "cache", Short: "Inspect or reclaim xgo-managed storage"}
	for _, name := range []string{"status", "gc"} {
		name := name
		c := &cobra.Command{Use: name, Args: cobra.NoArgs, Short: map[string]string{"status": "Show policy and managed storage (read only)", "gc": "Reclaim abandoned storage and enforce the disk budget"}[name], RunE: func(c *cobra.Command, args []string) error {
			flags := map[string]string{}
			for _, key := range append([]string{"config"}, storageKeys...) {
				if c.Flags().Changed(key) {
					flags[key], _ = c.Flags().GetString(key)
				}
			}
			project, _ := c.Flags().GetString("project-dir")
			home, _ := c.Flags().GetString("xgo-home")
			p, err := loadStoragePolicy(project, flags)
			if err != nil {
				return err
			}
			m, err := newStorageManager(p, home)
			if err != nil {
				return err
			}
			if name == "gc" {
				if !storageProcessGroups {
					return fmt.Errorf("automatic reclamation is supported on macOS/Linux; cache status is available on this platform")
				}
				if err = m.enforce(); err != nil {
					return err
				}
			}
			entries, err := m.entries()
			if err != nil {
				return err
			}
			var total int64
			for _, e := range entries {
				total += e.size
			}
			return storageJSON(struct {
				Policy           storagePolicy `json:"policy"`
				Bytes            int64         `json:"bytes"`
				Directories      int           `json:"directories"`
				SkippedOwnership int           `json:"skipped_unverified_ownership"`
				RuntimeGuard     bool          `json:"runtime_guard_supported"`
			}{m.policy, total, len(entries), m.skippedOwnership, storageProcessGroups})
		}}
		addStorageFlags(c)
		c.Flags().String("project-dir", "", "Project directory for config discovery")
		c.Flags().String("xgo-home", "", "xgo home directory")
		cache.AddCommand(c)
	}
	root.AddCommand(cache)
	return root
}

func addStorageFlags(c *cobra.Command) {
	c.Flags().String("cache-dir", "", "xgo-managed cache/session root (XGO_CACHE_DIR; default /tmp/xgo)")
	c.Flags().String("config", "", "Explicit Viper configuration file (or XGO_CONFIG)")
	c.Flags().String("cache-max-size", "20GiB", "Maximum xgo-managed storage (XGO_CACHE_MAX_SIZE)")
	c.Flags().String("cache-min-free", "10GiB", "Minimum free space on each managed volume (XGO_CACHE_MIN_FREE)")
	c.Flags().String("temp-max-age", "24h", "Maximum age of abandoned build sessions (XGO_TEMP_MAX_AGE)")
	c.Flags().String("disk-check-interval", "2s", "Disk guard polling interval, 100ms–1m (XGO_DISK_CHECK_INTERVAL)")
}
