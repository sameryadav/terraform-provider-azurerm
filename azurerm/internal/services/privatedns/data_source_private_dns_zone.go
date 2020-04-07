package privatedns

import (
	"context"
	"fmt"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/clients"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tags"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func dataSourceArmPrivateDnsZone() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceArmPrivateDnsZoneRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},

			"resource_group_name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"number_of_record_sets": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"max_number_of_record_sets": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"max_number_of_virtual_network_links": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"max_number_of_virtual_network_links_with_registration": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"name_servers": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"tags": tags.SchemaDataSource(),
		},
	}
}

func dataSourceArmPrivateDnsZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).PrivateDns.PrivateZonesClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	var (
		resp *privatedns.PrivateZone
	)
	if resourceGroup != "" {
		zone, err := client.Get(ctx, resourceGroup, name)
		if err != nil {
			if utils.ResponseWasNotFound(resp.Response) {
				return fmt.Errorf("Error: Private DNS Zone %q (Resource Group %q) was not found", name, resourceGroup)
			}
			return fmt.Errorf("error reading Private DNS Zone %q (Resource Group %q): %+v", name, resourceGroup, err)
		}
		resp = &zone
	} else {
		resourcesClient := meta.(*clients.Client).Resource.ResourcesClient

		zone, err := findPrivateZone(ctx, client, resourcesClient, name)
		if err != nil {
			return err
		}

		if zone == nil {
			return fmt.Errorf("Error: Private DNS Zone %q was not found", name)
		}

		resp = &zone.zone
		resourceGroup = zone.resourceGroup
	}

	d.Set("name", name)
	d.Set("resource_group_name", resourceGroup)

	//if props := resp.PrivateZoneProperties; props != nil {
	//	d.Set("number_of_record_sets", props.NumberOfRecordSets)
	//	d.Set("max_number_of_record_sets", props.MaxNumberOfRecordSets)
	//	d.Set("max_number_of_virtual_network_links", props.MaxNumberOfVirtualNetworkLinks)
	//	d.Set("max_number_of_virtual_network_links_with_registration", props.MaxNumberOfVirtualNetworkLinksWithRegistration)
	//}

	return tags.FlattenAndSet(d, resp.Tags)
}

type privateDnsZone struct {
	zone privatedns.PrivateZone
	resourceGroup string
}

func findPrivateZone(ctx context.Context, client *privatedns.PrivateZonesClient, resourcesClient *resources.Client, name string) (*privateDnsZone, error) {
	filter := fmt.Sprintf("resourceType eq 'Microsoft.Network/privateDnsZones' and name eq '%s'", name)
	privateZones, err := resourcesClient.List(ctx, filter, "", nil)

	if err != nil {
		return nil, fmt.Errorf("Error listing Private DNS Zones: %+v", err)
	}

	if len(privateZones.Values()) > 1 {
		return nil, fmt.Errorf("More than one Private DNS Zone found with name: %q", name)
	}

	for _, z := range privateZones.Values() {
		if z.ID == nil {
			continue
		}

		id, err := azure.ParseAzureResourceID(*z.ID)

		if err != nil {
			continue
		}

		zone, err := client.Get(ctx, id.ResourceGroup, name)

		if err != nil {
			return nil, fmt.Errorf("Error retrieving Private DNS Zone %q in resource group %q: %+v", name, id.ResourceGroup, err)
		}

		return &privateDnsZone{
			zone: zone,
			resourceGroup: id.ResourceGroup,
		}, nil
	}

	return nil, fmt.Errorf("No Private DNS Zones found with name: %q", name)
}
