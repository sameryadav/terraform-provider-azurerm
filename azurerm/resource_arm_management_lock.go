package azurerm

import (
	"fmt"
	"log"
	"strings"

	"regexp"

	"github.com/Azure/azure-sdk-for-go/arm/resources/locks"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmManagementLock() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmManagementLockCreateUpdate,
		Read:   resourceArmManagementLockRead,
		Delete: resourceArmManagementLockDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateArmManagementLockName,
			},

			"scope": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"lock_level": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(locks.CanNotDelete),
					string(locks.ReadOnly),
				}, false),
			},

			"notes": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(0, 512),
			},
		},
	}
}

func resourceArmManagementLockCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).managementLocksClient
	log.Printf("[INFO] preparing arguments for AzureRM Management Lock creation.")

	name := d.Get("name").(string)
	scope := d.Get("scope").(string)
	lockLevel := d.Get("lock_level").(string)
	notes := d.Get("notes").(string)

	lock := locks.ManagementLockObject{
		ManagementLockProperties: &locks.ManagementLockProperties{
			Level: locks.LockLevel(lockLevel),
			Notes: utils.String(notes),
		},
	}

	_, err := client.CreateOrUpdateByScope(scope, name, lock)
	if err != nil {
		return err
	}

	read, err := client.GetByScope(scope, name)
	if err != nil {
		return err
	}

	if read.ID == nil {
		return fmt.Errorf("Cannot read ID of AzureRM Management Lock %q (Scope %q)", name, scope)
	}

	d.SetId(*read.ID)
	return resourceArmManagementLockRead(d, meta)
}

func resourceArmManagementLockRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).managementLocksClient

	id, err := parseAzureRMLockId(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.GetByScope(id.Scope, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error making Read request on AzureRM Management Lock %q (Scope %q): %+v", id.Name, id.Scope, err)
	}

	d.Set("name", resp.Name)
	d.Set("scope", id.Scope)

	if props := resp.ManagementLockProperties; props != nil {
		d.Set("lock_level", string(props.Level))
		d.Set("notes", props.Notes)
	}

	return nil
}

func resourceArmManagementLockDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).managementLocksClient

	id, err := parseAzureRMLockId(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.DeleteByScope(id.Scope, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp) {
			return nil
		}

		return fmt.Errorf("Error issuing AzureRM delete request for Management Lock %q (Scope %q): %+v", id.Name, id.Scope, err)
	}

	return nil
}

type AzureManagementLockId struct {
	Scope string
	Name  string
}

func parseAzureRMLockId(id string) (*AzureManagementLockId, error) {
	segments := strings.Split(id, "/providers/Microsoft.Authorization/locks/")
	if len(segments) != 2 {
		return nil, fmt.Errorf("Expected ID to be in the format `{scope}/providers/Microsoft.Authorization/locks/{name} - got %d segments", len(segments))
	}

	scope := segments[0]
	name := segments[1]
	lockId := AzureManagementLockId{
		Scope: scope,
		Name:  name,
	}
	return &lockId, nil
}

func validateArmManagementLockName(v interface{}, k string) (ws []string, es []error) {
	input := v.(string)

	if !regexp.MustCompile(`[A-Za-z0-9-_]`).MatchString(input) {
		es = append(es, fmt.Errorf("%s can only consist of alphanumeric characters, dashes and underscores", k))
	}

	if len(input) >= 260 {
		es = append(es, fmt.Errorf("%s can only be a maximum of 260 characters", k))
	}

	return
}
