package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amdl/internal/updater"
	"amdl/internal/version"
	"amdl/utils/structs"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type maintenanceFlags struct {
	version, update, check, yes, force, migrate bool
	liteServer                                  string
}

func registerMaintenanceFlags() *maintenanceFlags {
	f := &maintenanceFlags{}
	pflag.BoolVarP(&f.version, "version", "v", false, "Print version information and exit")
	pflag.BoolVarP(&f.update, "update", "U", false, "Self-update from this fork's releases")
	pflag.BoolVar(&f.check, "check-update", false, "Check for a release without downloading")
	pflag.BoolVarP(&f.yes, "yes", "y", false, "Accept defaults during update/config migration")
	pflag.BoolVar(&f.force, "force-update", false, "Reinstall the latest release with --update")
	pflag.BoolVar(&f.migrate, "migrate-config", false, "Add missing options while preserving configured values")
	pflag.StringVar(&f.liteServer, "lite-server", "", "wrapper-lite HTTP endpoint for this run")
	return f
}

func programName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return filepath.Base(path)
}

func explicitConfigRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--config" || arg == "-c" || strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c=") {
			return true
		}
	}
	return false
}

// migrationTemplate copies existing flat values into the corresponding template
// defaults, so adding nested keys cannot override a user's credentials or paths.
func migrationTemplate(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfigExample, nil
	}
	if err != nil {
		return "", err
	}
	data, err = normalizeConfigData(data)
	if err != nil {
		return "", err
	}
	var raw, template map[interface{}]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	if err := yaml.Unmarshal([]byte(defaultConfigExample), &template); err != nil {
		return "", err
	}
	for flat, nested := range legacyConfigKeys {
		if _, ok := configValue(raw, nested); ok {
			continue
		}
		if value, ok := raw[flat]; ok {
			setNestedConfigValue(template, nested, value)
		}
	}
	result, err := yaml.Marshal(template)
	return string(result), err
}

func setNestedConfigValue(raw map[interface{}]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		next, ok := raw[part].(map[interface{}]interface{})
		if !ok {
			next = map[interface{}]interface{}{}
			raw[part] = next
		}
		raw = next
	}
	raw[parts[len(parts)-1]] = value
}

func runMaintenance(f *maintenanceFlags) bool {
	var err error
	switch {
	case f.version:
		fmt.Println(version.Info())
	case f.migrate:
		var template string
		template, err = migrationTemplate(config_path)
		if err == nil {
			updater.DefaultConfigExample = template
			err = updater.RunConfigMigration(config_path, f.yes)
		}
	case f.check:
		err = updater.CheckUpdateOnly(Config.Proxy)
	case f.update:
		err = updater.ExecuteSelfUpdate(config_path, Config.Proxy, f.yes, f.force)
	default:
		return false
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return true
}

// Explicit CLI values override either supported YAML schema before validation.
func applyCLIConfigOverrides(cfg *structs.ConfigSet, f *maintenanceFlags) {
	if pflag.CommandLine.Changed("lite-server") {
		cfg.LiteServer = f.liteServer
	}
	if pflag.CommandLine.Changed("storefront") {
		cfg.Storefront = id_storefront
	}
	if pflag.CommandLine.Changed("alac-max") {
		cfg.AlacMax = *alac_max
	}
	if pflag.CommandLine.Changed("atmos-max") {
		cfg.AtmosMax = *atmos_max
	}
	if pflag.CommandLine.Changed("aac-type") {
		cfg.AacType = *aac_type
	}
	if pflag.CommandLine.Changed("mv-audio-type") {
		cfg.MVAudioType = *mv_audio_type
	}
	if pflag.CommandLine.Changed("mv-max") {
		cfg.MVMax = *mv_max
	}
}
