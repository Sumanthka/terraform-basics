package applicationinsights

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/appinsights/mgmt/2015-05-01/insights"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/clients"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/applicationinsights/parse"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/pluginsdk"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceApplicationInsightsSmartDetectionRule() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceApplicationInsightsSmartDetectionRuleUpdate,
		Read:   resourceApplicationInsightsSmartDetectionRuleRead,
		Update: resourceApplicationInsightsSmartDetectionRuleUpdate,
		Delete: resourceApplicationInsightsSmartDetectionRuleDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"Slow page load time",
					"Slow server response time",
					"Long dependency duration",
					"Degradation in server response time",
					"Degradation in dependency duration",
					// The below rules are currently preview and may change in future
					"Degradation in trace severity ratio",
					"Abnormal rise in exception volume",
					"Potential memory leak detected",
					"Potential security issue detected",
					"Abnormal rise in daily data volume",
				}, false),
				DiffSuppressFunc: smartDetectionRuleNameDiff,
			},

			"application_insights_id": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateResourceID,
			},

			"enabled": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
				Default:  true,
			},

			"send_emails_to_subscription_owners": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
				Default:  true,
			},

			"additional_email_recipients": {
				Type:     pluginsdk.TypeSet,
				Optional: true,
				Elem:     &pluginsdk.Schema{Type: pluginsdk.TypeString},
			},
		},
	}
}

func resourceApplicationInsightsSmartDetectionRuleUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AppInsights.SmartDetectionRuleClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for AzureRM Application Insights Samrt Detection Rule update.")

	// The Smart Detection Rule name from the UI doesn't match what the API accepts.
	// We'll have the user submit what the name looks like in the UI and convert it behind the scenes to match what the API accepts
	name := convertUiNameToApiName(d.Get("name"))

	appInsightsID := d.Get("application_insights_id").(string)

	id, err := parse.ComponentID(appInsightsID)
	if err != nil {
		return err
	}

	smartDetectionRuleProperties := insights.ApplicationInsightsComponentProactiveDetectionConfiguration{
		Name:                           &name,
		Enabled:                        utils.Bool(d.Get("enabled").(bool)),
		SendEmailsToSubscriptionOwners: utils.Bool(d.Get("send_emails_to_subscription_owners").(bool)),
		CustomEmails:                   utils.ExpandStringSlice(d.Get("additional_email_recipients").(*pluginsdk.Set).List()),
	}

	_, err = client.Update(ctx, id.ResourceGroup, id.Name, name, smartDetectionRuleProperties)
	if err != nil {
		return fmt.Errorf("updating Application Insights Smart Detection Rule %q (Application Insights %q): %+v", name, id.String(), err)
	}

	d.SetId(fmt.Sprintf("%s/SmartDetectionRule/%s", id.ID(), name))

	return resourceApplicationInsightsSmartDetectionRuleRead(d, meta)
}

func resourceApplicationInsightsSmartDetectionRuleRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AppInsights.SmartDetectionRuleClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SmartDetectionRuleID(d.Id())
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Reading AzureRM Application Insights Smart Detection Rule %q", id.String())

	result, err := client.Get(ctx, id.ResourceGroup, id.ComponentName, id.SmartDetectionRuleName)
	if err != nil {
		if utils.ResponseWasNotFound(result.Response) {
			log.Printf("[WARN] AzureRM Application Insights Smart Detection Rule  %q not found, removing from state", id.String())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on AzureRM Application Insights Smart Detection Rule %q: %+v", id.String(), err)
	}

	d.Set("name", result.Name)
	d.Set("application_insights_id", parse.NewComponentID(id.SubscriptionId, id.ResourceGroup, id.ComponentName).ID())
	d.Set("enabled", result.Enabled)
	d.Set("send_emails_to_subscription_owners", result.SendEmailsToSubscriptionOwners)
	d.Set("additional_email_recipients", utils.FlattenStringSlice(result.CustomEmails))
	return nil
}

func resourceApplicationInsightsSmartDetectionRuleDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AppInsights.SmartDetectionRuleClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SmartDetectionRuleID(d.Id())
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] reseting AzureRM Application Insights Smart Detection Rule %q", id.String())

	result, err := client.Get(ctx, id.ResourceGroup, id.ComponentName, id.SmartDetectionRuleName)
	if err != nil {
		if utils.ResponseWasNotFound(result.Response) {
			log.Printf("[WARN] AzureRM Application Insights Smart Detection Rule %q not found, removing from state", id.String())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on AzureRM Application Insights Smart Detection Rule %q: %+v", id.String(), err)
	}

	smartDetectionRuleProperties := insights.ApplicationInsightsComponentProactiveDetectionConfiguration{
		Name:                           utils.String(id.SmartDetectionRuleName),
		Enabled:                        result.RuleDefinitions.IsEnabledByDefault,
		SendEmailsToSubscriptionOwners: result.RuleDefinitions.SupportsEmailNotifications,
		CustomEmails:                   utils.ExpandStringSlice([]interface{}{}),
	}

	// Application Insights defaults all the Smart Detection Rules so if a user wants to delete a rule, we'll update it back to it's default values.
	_, err = client.Update(ctx, id.ResourceGroup, id.ComponentName, id.SmartDetectionRuleName, smartDetectionRuleProperties)
	if err != nil {
		if utils.ResponseWasNotFound(result.Response) {
			return nil
		}
		return fmt.Errorf("issuing AzureRM reset update request for Application Insights Smart Detection Rule %q: %+v", id.String(), err)
	}

	return nil
}

// The Smart Detection Rule name from the UI doesn't match what the API accepts.
// This Diff checks if the old and new name match when converted to the API version of the name
func smartDetectionRuleNameDiff(_, old string, new string, _ *pluginsdk.ResourceData) bool {
	apiNew := convertUiNameToApiName(new)

	return strings.EqualFold(old, apiNew)
}

func convertUiNameToApiName(uiName interface{}) string {
	var apiName string
	switch uiName.(string) {
	case "Slow page load time":
		apiName = "slowpageloadtime"
	case "Slow server response time":
		apiName = "slowserverresponsetime"
	case "Long dependency duration":
		apiName = "longdependencyduration"
	case "Degradation in server response time":
		apiName = "degradationinserverresponsetime"
	case "Degradation in dependency duration":
		apiName = "degradationindependencyduration"
	case "Degradation in trace severity ratio":
		apiName = "extension_traceseveritydetector"
	case "Abnormal rise in exception volume":
		apiName = "extension_exceptionchangeextension"
	case "Potential memory leak detected":
		apiName = "extension_memoryleakextension"
	case "Potential security issue detected":
		apiName = "extension_securityextensionspackage"
	case "Abnormal rise in daily data volume":
		apiName = "extension_billingdatavolumedailyspikeextension"
	}
	return apiName
}
