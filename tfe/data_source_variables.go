package tfe

import (
	"fmt"
	"log"

	tfe "github.com/hashicorp/go-tfe"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTFEWorkspaceVariables() *schema.Resource {
	varSchema := map[string]*schema.Schema{
		"category": {
			Description: "The category of the variable (terraform or environment).",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"hcl": {
			Description: "If the variable is marked as HCL or not.",
			Type:        schema.TypeBool,
			Computed:    true,
		},
		"id": {
			Description: "The variable ID.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"name": {
			Description: "The variable key name.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"sensitive": {
			Description: "If the variable is marked as sensitive or not.",
			Type:        schema.TypeBool,
			Computed:    true,
		},
		"value": {
			Description: "The variable value. If the variable is sensitive this value will be empty.",
			Type:        schema.TypeString,
			Computed:    true,
			Sensitive:   true,
		},
	}
	return &schema.Resource{
		Description: "This data source is used to retrieve all variables defined in a specified workspace.",
		Read:        dataSourceVariableRead,

		Schema: map[string]*schema.Schema{
			"env": {
				Description: "List containing environment variables configured on the workspace.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: varSchema,
				},
			},
			"terraform": {
				Description: "List containing terraform variables configured on the workspace.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: varSchema,
				},
			},
			"variables": {
				Description: "List containing all terraform and environment variables configured on the workspace.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: varSchema,
				},
			},
			"workspace_id": {
				Description:  "ID of the workspace.",
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"workspace_id", "variable_set_id"},
			},
			"variable_set_id": {
				Description:  "ID of the workspace.",
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"workspace_id", "variable_set_id"},
			},
		},
	}
}

func dataSourceVariableRead(d *schema.ResourceData, meta interface{}) error {
	// Switch to variable set variable logic
	_, variableSetIdProvided := d.GetOk("variable_set_id")
	if variableSetIdProvided {
		return dataSourceVariableSetVariableRead(d, meta)
	}

	tfeClient := meta.(*tfe.Client)

	// Get the name and organization.
	workspaceID := d.Get("workspace_id").(string)

	log.Printf("[DEBUG] Read configuration of workspace: %s", workspaceID)

	totalEnvVariables := make([]interface{}, 0)
	totalTerraformVariables := make([]interface{}, 0)

	options := &tfe.VariableListOptions{}

	for {
		variableList, err := tfeClient.Variables.List(ctx, workspaceID, options)
		if err != nil {
			return fmt.Errorf("Error retrieving variable list: %w", err)
		}
		terraformVars := make([]interface{}, 0)
		envVars := make([]interface{}, 0)
		for _, variable := range variableList.Items {
			result := make(map[string]interface{})
			result["id"] = variable.ID
			result["category"] = variable.Category
			result["hcl"] = variable.HCL
			result["name"] = variable.Key
			result["sensitive"] = variable.Sensitive
			result["value"] = variable.Value
			if variable.Category == "terraform" {
				terraformVars = append(terraformVars, result)
			} else if variable.Category == "env" {
				envVars = append(envVars, result)
			}
		}

		totalEnvVariables = append(totalEnvVariables, envVars...)
		totalTerraformVariables = append(totalTerraformVariables, terraformVars...)

		// Exit the loop when we've seen all pages.
		if variableList.CurrentPage >= variableList.TotalPages {
			break
		}

		// Update the page number to get the next page.
		options.PageNumber = variableList.NextPage
	}

	d.SetId(fmt.Sprintf("variables/%v", workspaceID))
	d.Set("variables", append(totalTerraformVariables, totalEnvVariables...))
	d.Set("terraform", totalTerraformVariables)
	d.Set("env", totalEnvVariables)
	return nil
}

func dataSourceVariableSetVariableRead(d *schema.ResourceData, meta interface{}) error {
	tfeClient := meta.(*tfe.Client)

	// Get the id.
	variableSetId := d.Get("variable_set_id").(string)

	log.Printf("[DEBUG] Read configuration of variable set: %s", variableSetId)

	totalEnvVariables := make([]interface{}, 0)
	totalTerraformVariables := make([]interface{}, 0)

	options := tfe.VariableSetVariableListOptions{}

	for {
		variableList, err := tfeClient.VariableSetVariables.List(ctx, variableSetId, &options)
		if err != nil {
			return fmt.Errorf("Error retrieving variable list: %w", err)
		}
		terraformVars := make([]interface{}, 0)
		envVars := make([]interface{}, 0)
		for _, variable := range variableList.Items {
			result := make(map[string]interface{})
			result["id"] = variable.ID
			result["category"] = variable.Category
			result["hcl"] = variable.HCL
			result["name"] = variable.Key
			result["sensitive"] = variable.Sensitive
			result["value"] = variable.Value
			if variable.Category == "terraform" {
				terraformVars = append(terraformVars, result)
			} else if variable.Category == "env" {
				envVars = append(envVars, result)
			}
		}

		totalEnvVariables = append(totalEnvVariables, envVars...)
		totalTerraformVariables = append(totalTerraformVariables, terraformVars...)

		// Exit the loop when we've seen all pages.
		if variableList.CurrentPage >= variableList.TotalPages {
			break
		}

		// Update the page number to get the next page.
		options.PageNumber = variableList.NextPage
	}

	d.SetId(fmt.Sprintf("variables/%v", variableSetId))
	d.Set("variables", append(totalTerraformVariables, totalEnvVariables...))
	d.Set("terraform", totalTerraformVariables)
	d.Set("env", totalEnvVariables)
	return nil
}
