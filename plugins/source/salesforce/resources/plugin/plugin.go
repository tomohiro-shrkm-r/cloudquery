package plugin

import (
	"github.com/cloudquery/cloudquery/plugins/source/salesforce/client"
	"github.com/cloudquery/cloudquery/plugins/source/salesforce/resources/services"
	"github.com/cloudquery/plugin-sdk/v2/plugins/source"
	"github.com/cloudquery/plugin-sdk/v2/schema"
)

var (
	Version = "Development"
)

func Plugin() *source.Plugin {
	return source.NewPlugin(
		"salesforce",
		Version,
		[]*schema.Table{
			services.Objects(),
		},
		client.Configure,
	)
}
