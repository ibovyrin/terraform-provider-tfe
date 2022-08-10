package tfe

import (
	"fmt"
	"strings"

	tfe "github.com/hashicorp/go-tfe"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTFEWorkspaceIDs() *schema.Resource {
	return &schema.Resource{
		Description: "Use this data source to get a map of workspace IDs.",

		Read: dataSourceTFEWorkspaceIDsRead,

		Schema: map[string]*schema.Schema{
			"names": {
				Description: " A list of workspace names to search for. Names that don't match a real workspace will be omitted from the results, but are not an error." +
					"\n\n  To select _all_ workspaces for an organization, provide a list with a single asterisk, like `[\"*\"]`. No other use of wildcards is supported.",
				Type:         schema.TypeList,
				Elem:         &schema.Schema{Type: schema.TypeString},
				Optional:     true,
				AtLeastOneOf: []string{"names", "tag_names"},
			},

			"tag_names": {
				Description: "A list of tag names to search for.",
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
			},

			"exclude_tags": {
				Description: "A list of tag names to exclude when searching.",
				Type:        schema.TypeSet,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
			},

			"organization": {
				Description: "Name of the organization.",
				Type:        schema.TypeString,
				Required:    true,
			},

			"ids": {
				Description: "A map of workspace names and their opaque, immutable IDs, which look like `ws-<RANDOM STRING>`.",
				Type:        schema.TypeMap,
				Computed:    true,
			},

			"full_names": {
				Description: "A map of workspace names and their full names, which look like `<ORGANIZATION>/<WORKSPACE>`.",
				Type:        schema.TypeMap,
				Computed:    true,
			},
		},
	}
}

func dataSourceTFEWorkspaceIDsRead(d *schema.ResourceData, meta interface{}) error {
	tfeClient := meta.(*tfe.Client)

	// Get the organization.
	organization := d.Get("organization").(string)

	// Create a map with all the names we are looking for.
	var id string
	names := make(map[string]bool)
	for _, name := range d.Get("names").([]interface{}) {
		// ignore empty strings
		if name == nil {
			continue
		}

		id += name.(string)
		names[name.(string)] = true
	}
	isWildcard := names["*"]

	// Create two maps to hold the results.
	fullNames := make(map[string]string, len(names))
	ids := make(map[string]string, len(names))

	options := &tfe.WorkspaceListOptions{}

	excludeTagLookupMap := make(map[string]bool)
	var excludeTagBuf strings.Builder
	for _, excludedTag := range d.Get("exclude_tags").(*schema.Set).List() {
		if exTag, ok := excludedTag.(string); ok && len(strings.TrimSpace(exTag)) != 0 {
			excludeTagLookupMap[exTag] = true

			if excludeTagBuf.Len() > 0 {
				excludeTagBuf.WriteByte(',')
			}
			excludeTagBuf.WriteString(exTag)
		}
	}

	if excludeTagBuf.Len() > 0 {
		options.ExcludeTags = excludeTagBuf.String()
	}

	// Create a search string with all the tag names we are looking for.
	var tagSearchParts []string
	for _, tagName := range d.Get("tag_names").([]interface{}) {
		if name, ok := tagName.(string); ok && len(strings.TrimSpace(name)) != 0 {
			id += name // add to the state id
			tagSearchParts = append(tagSearchParts, name)
		}
	}
	if len(tagSearchParts) > 0 {
		tagSearch := strings.Join(tagSearchParts, ",")
		options.Tags = tagSearch
	}

	hasOnlyTags := len(tagSearchParts) > 0 && len(names) == 0

	for {
		wl, err := tfeClient.Workspaces.List(ctx, organization, options)
		if err != nil {
			return fmt.Errorf("Error retrieving workspaces: %w", err)
		}

		for _, w := range wl.Items {
			nameIncluded := isWildcard || names[w.Name]
			// fallback for tfe instances that don't yet support exclude-tags
			hasExcludedTag := false
			for _, tag := range w.TagNames {
				if _, ok := excludeTagLookupMap[tag]; ok {
					hasExcludedTag = true
					break
				}
			}
			if (hasOnlyTags || nameIncluded) && !hasExcludedTag {
				fullNames[w.Name] = organization + "/" + w.Name
				ids[w.Name] = w.ID
			}
		}

		// Exit the loop when we've seen all pages.
		if wl.CurrentPage >= wl.TotalPages {
			break
		}

		// Update the page number to get the next page.
		options.PageNumber = wl.NextPage
	}

	d.Set("ids", ids)
	d.Set("full_names", fullNames)
	d.SetId(fmt.Sprintf("%s/%d", organization, schema.HashString(id)))

	return nil
}
