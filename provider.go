package main

import (
	"github.com/aristanetworks/goarista/gnmi"
	"github.com/hashicorp/terraform/helper/schema"
)

// Provider returns a terraform.ResourceProvider.
func Provider() *schema.Provider {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"address": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADDRESS", ""),
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("USERNAME", ""),
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PASSWORD", ""),
			},
			"tls": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"gnmi_interface": resourceInterface(),
		},
	}

	p.ConfigureFunc = providerConfigure(p)

	return p
}

func providerConfigure(p *schema.Provider) schema.ConfigureFunc {
	return func(d *schema.ResourceData) (interface{}, error) {
		cfg := &gnmi.Config{
			Addr:     d.Get("address").(string),
			TLS:      d.Get("tls").(bool),
			Username: d.Get("username").(string),
			Password: d.Get("password").(string),
		}

		client, err := getGNMIClient(cfg)
		if err != nil {
			return nil, err
		}

		return client, nil
	}

}
