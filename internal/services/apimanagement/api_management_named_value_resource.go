package apimanagement

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/apimanagement/mgmt/2020-12-01/apimanagement"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/apimanagement/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/apimanagement/schemaz"
	keyVaultValidate "github.com/hashicorp/terraform-provider-azurerm/internal/services/keyvault/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func resourceApiManagementNamedValue() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceApiManagementNamedValueCreateUpdate,
		Read:   resourceApiManagementNamedValueRead,
		Update: resourceApiManagementNamedValueCreateUpdate,
		Delete: resourceApiManagementNamedValueDelete,
		// TODO: replace this with an importer which validates the ID during import
		Importer: pluginsdk.DefaultImporter(),

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": schemaz.SchemaApiManagementChildName(),

			"resource_group_name": azure.SchemaResourceGroupName(),

			"api_management_name": schemaz.SchemaApiManagementName(),

			"display_name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"value_from_key_vault": {
				Type:         pluginsdk.TypeList,
				Optional:     true,
				MaxItems:     1,
				ExactlyOneOf: []string{"value", "value_from_key_vault"},
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"secret_id": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: keyVaultValidate.NestedItemIdWithOptionalVersion,
						},
						"identity_client_id": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.IsUUID,
						},
					},
				},
			},

			"value": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				Sensitive:    true,
				ValidateFunc: validation.StringIsNotEmpty,
				ExactlyOneOf: []string{"value", "value_from_key_vault"},
			},

			"secret": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
				Default:  false,
			},

			"tags": {
				Type:     pluginsdk.TypeList,
				Optional: true,
				Elem: &pluginsdk.Schema{
					Type: pluginsdk.TypeString,
				},
			},
		},
	}
}

func resourceApiManagementNamedValueCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ApiManagement.NamedValueClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	serviceName := d.Get("api_management_name").(string)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, resourceGroup, serviceName, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf(" checking for presence of existing Property %q (API Management Service %q / Resource Group %q): %s", name, serviceName, resourceGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_api_management_property", *existing.ID)
		}
	}

	parameters := apimanagement.NamedValueCreateContract{
		NamedValueCreateContractProperties: &apimanagement.NamedValueCreateContractProperties{
			DisplayName: utils.String(d.Get("display_name").(string)),
			Secret:      utils.Bool(d.Get("secret").(bool)),
			KeyVault:    expandApiManagementNamedValueKeyVault(d.Get("value_from_key_vault").([]interface{})),
		},
	}

	if v, ok := d.GetOk("value"); ok {
		parameters.NamedValueCreateContractProperties.Value = utils.String(v.(string))
	}

	if tags, ok := d.GetOk("tags"); ok {
		parameters.NamedValueCreateContractProperties.Tags = utils.ExpandStringSlice(tags.([]interface{}))
	}

	future, err := client.CreateOrUpdate(ctx, resourceGroup, serviceName, name, parameters, "")
	if err != nil {
		return fmt.Errorf(" creating or updating Property %q (Resource Group %q / API Management Service %q): %+v", name, resourceGroup, serviceName, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting on creating/updating Property %q (Resource Group %q / API Management Service %q): %+v", name, resourceGroup, serviceName, err)
	}

	resp, err := client.Get(ctx, resourceGroup, serviceName, name)
	if err != nil {
		return fmt.Errorf(" retrieving Property %q (Resource Group %q / API Management Service %q): %+v", name, resourceGroup, serviceName, err)
	}
	if resp.ID == nil {
		return fmt.Errorf("Cannot read ID for Property %q (Resource Group %q / API Management Service %q)", name, resourceGroup, serviceName)
	}
	d.SetId(*resp.ID)

	return resourceApiManagementNamedValueRead(d, meta)
}

func resourceApiManagementNamedValueRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ApiManagement.NamedValueClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.NamedValueID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	serviceName := id.ServiceName
	name := id.Name

	resp, err := client.Get(ctx, resourceGroup, serviceName, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[DEBUG] Property %q (Resource Group %q / API Management Service %q) was not found - removing from state!", name, resourceGroup, serviceName)
			d.SetId("")
			return nil
		}

		return fmt.Errorf(" making Read request for Property %q (Resource Group %q / API Management Service %q): %+v", name, resourceGroup, serviceName, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resourceGroup)
	d.Set("api_management_name", serviceName)

	if properties := resp.NamedValueContractProperties; properties != nil {
		d.Set("display_name", properties.DisplayName)
		d.Set("secret", properties.Secret)
		// API will not return `value` when `secret` is `true`, in which case we shall not set the `value`. Refer to the issue : #6688
		if properties.Secret != nil && !*properties.Secret {
			d.Set("value", properties.Value)
		}
		if err := d.Set("value_from_key_vault", flattenApiManagementNamedValueKeyVault(properties.KeyVault)); err != nil {
			return fmt.Errorf("setting `value_from_key_vault`: %+v", err)
		}
		d.Set("tags", properties.Tags)
	}

	return nil
}

func resourceApiManagementNamedValueDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ApiManagement.NamedValueClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.NamedValueID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	serviceName := id.ServiceName
	name := id.Name

	if resp, err := client.Delete(ctx, resourceGroup, serviceName, name, ""); err != nil {
		if !utils.ResponseWasNotFound(resp) {
			return fmt.Errorf(" deleting Property %q (Resource Group %q / API Management Service %q): %+v", name, resourceGroup, serviceName, err)
		}
	}

	return nil
}

func expandApiManagementNamedValueKeyVault(inputs []interface{}) *apimanagement.KeyVaultContractCreateProperties {
	if len(inputs) == 0 {
		return nil
	}
	input := inputs[0].(map[string]interface{})

	return &apimanagement.KeyVaultContractCreateProperties{
		SecretIdentifier: utils.String(input["secret_id"].(string)),
		IdentityClientID: utils.String(input["identity_client_id"].(string)),
	}
}

func flattenApiManagementNamedValueKeyVault(input *apimanagement.KeyVaultContractProperties) []interface{} {
	if input == nil {
		return []interface{}{}
	}

	var secretId, clientId string
	if input.SecretIdentifier != nil {
		secretId = *input.SecretIdentifier
	}

	if input.IdentityClientID != nil {
		clientId = *input.IdentityClientID
	}

	return []interface{}{
		map[string]interface{}{
			"secret_id":          secretId,
			"identity_client_id": clientId,
		},
	}
}
