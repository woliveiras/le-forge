// Package environmentpolicy defines installation-owned restrictions. A graph,
// repository definition or developer approval cannot override these settings.
package environmentpolicy

import (
	"fmt"
	"slices"

	"github.com/woliveiras/le-forge/server/internal/domain"
)

type Policy struct {
	SetupNetworks         []string `yaml:"setupNetworks" json:"setupNetworks"`
	RunNetworks           []string `yaml:"runNetworks" json:"runNetworks"`
	DisableAgentProposals bool     `yaml:"disableAgentProposals" json:"disableAgentProposals"`
	DisableDockerfiles    bool     `yaml:"disableDockerfiles" json:"disableDockerfiles"`
	DisableSetup          bool     `yaml:"disableSetup" json:"disableSetup"`
	DisableCompanions     bool     `yaml:"disableCompanions" json:"disableCompanions"`
}

// Nil lists preserve existing installations. An explicit empty list is invalid.
func networks(list []string) []string {
	if list == nil {
		return []string{"none", "bridge"}
	}
	return list
}

func (p Policy) Validate() error {
	for field, list := range map[string][]string{"setupNetworks": p.SetupNetworks, "runNetworks": p.RunNetworks} {
		if len(networks(list)) == 0 {
			return fmt.Errorf("environment policy %s must not be empty", field)
		}
		for _, network := range networks(list) {
			if network != "none" && network != "bridge" {
				return fmt.Errorf("environment policy %s supports only none and bridge", field)
			}
		}
	}
	return nil
}

func (p Policy) RunNetwork(network string) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if network == "" {
		network = "none"
	}
	if !slices.Contains(networks(p.RunNetworks), network) {
		return fmt.Errorf("installation environment policy denies execution network; select an allowed network")
	}
	return nil
}

func (p Policy) Check(e *domain.EnvironmentSpec) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if e == nil {
		return nil
	}
	setup := e.Network.Setup
	if setup == "" {
		setup = "bridge"
	}
	if !slices.Contains(networks(p.SetupNetworks), setup) {
		return fmt.Errorf("installation environment policy denies preparation network; explicitly select an allowed network")
	}
	if err := p.RunNetwork(e.Network.Run); err != nil {
		return err
	}
	if p.DisableAgentProposals && e.Origin != nil && e.Origin.Source == "agent" {
		return fmt.Errorf("installation environment policy disables agent proposals")
	}
	if p.DisableDockerfiles && e.Dockerfile != "" {
		return fmt.Errorf("installation environment policy disables Dockerfiles")
	}
	if p.DisableSetup && len(e.Setup) > 0 {
		return fmt.Errorf("installation environment policy disables setup commands")
	}
	if p.DisableCompanions && len(e.Companions) > 0 {
		return fmt.Errorf("installation environment policy disables companion services")
	}
	return nil
}
