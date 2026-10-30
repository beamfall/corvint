package gokernel

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

type hostSchema struct {
	Version           string                `json:"version"`
	DegradationPolicy hostDegradationPolicy `json:"degradationPolicy"`
	Hosts             []hostDefinition      `json:"hosts"`
}

type hostDegradationPolicy struct {
	Match          string `json:"match"`
	OnUnrecognised string `json:"onUnrecognised"`
}

type hostDefinition struct {
	Name string `json:"name"`
}

//go:embed host-schema.json
var embeddedHostSchemaJSON []byte

var (
	embeddedHostSchema = mustHostSchema(embeddedHostSchemaJSON)
	harnessHosts       = hostNames(embeddedHostSchema)
	harnessHostSet     = hostSet(harnessHosts)
)

func mustHostSchema(raw []byte) hostSchema {
	var schema hostSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(fmt.Sprintf("invalid embedded host schema: %v", err))
	}
	if schema.Version != "0" {
		panic(fmt.Sprintf("invalid embedded host schema version %q", schema.Version))
	}
	if schema.DegradationPolicy.Match != "subset-of-recognised" || schema.DegradationPolicy.OnUnrecognised != "refuse" {
		panic("invalid embedded host schema degradation policy")
	}
	if len(schema.Hosts) == 0 {
		panic("embedded host schema must declare at least one host")
	}
	seen := make(map[string]struct{}, len(schema.Hosts))
	for _, host := range schema.Hosts {
		if _, err := token(host.Name, "host"); err != nil {
			panic(fmt.Sprintf("invalid embedded host schema: %v", err))
		}
		if _, duplicate := seen[host.Name]; duplicate {
			panic(fmt.Sprintf("duplicate embedded host %q", host.Name))
		}
		seen[host.Name] = struct{}{}
	}
	return schema
}

func hostNames(schema hostSchema) []string {
	names := make([]string, 0, len(schema.Hosts))
	for _, host := range schema.Hosts {
		names = append(names, host.Name)
	}
	slices.Sort(names)
	return names
}

func hostSet(hosts []string) map[string]struct{} {
	set := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		set[host] = struct{}{}
	}
	return set
}

// HarnessHosts returns the host identifiers in the embedded schema.
func HarnessHosts() []string {
	return slices.Clone(harnessHosts)
}

// KnownHarnessHost reports whether the embedded schema admits host.
func KnownHarnessHost(host string) bool {
	_, ok := harnessHostSet[host]
	return ok
}
