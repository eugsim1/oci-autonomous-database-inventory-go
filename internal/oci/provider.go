package oci

import (
	"fmt"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/config"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

func configurationProvider(cfg config.Config) (common.ConfigurationProvider, string, error) {
	var provider common.ConfigurationProvider
	var err error

	switch cfg.AuthMode {
	case config.AuthAPIKey:
		provider = common.CustomProfileConfigProvider(cfg.ConfigFile, cfg.Profile)
	case config.AuthInstancePrincipal:
		provider, err = auth.InstancePrincipalConfigurationProvider()
		if err != nil {
			return nil, "", fmt.Errorf("create instance principal provider: %w", err)
		}
	case config.AuthResourcePrincipal:
		provider, err = auth.ResourcePrincipalConfigurationProvider()
		if err != nil {
			return nil, "", fmt.Errorf("create resource principal provider: %w", err)
		}
	default:
		return nil, "", fmt.Errorf("unsupported authentication mode %q", cfg.AuthMode)
	}

	tenancyID := cfg.TenancyOCID
	if tenancyID == "" {
		tenancyID, err = provider.TenancyOCID()
		if err != nil {
			return nil, "", fmt.Errorf("resolve tenancy OCID from %s provider: %w", cfg.AuthMode, err)
		}
	}
	return provider, tenancyID, nil
}
