package cmd

import (
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

// resourceListOptions covers the common query parameters on the new experiment
// and workflow collections. API field selection is separate from --fields,
// which filters the CLI's table columns.
type resourceListOptions struct {
	page, limit                                                 int
	search, searchField, orderBy, direction, startDate, endDate string
	apiFields                                                   []string
}

func (o *resourceListOptions) flags(cmd *cobra.Command, searchField, searchHelp string, fieldSelection bool) {
	cmd.Flags().IntVar(&o.page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&o.limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&o.search, "search", "", searchHelp)
	cmd.Flags().StringVar(&o.searchField, "search-field", searchField, searchHelp)
	cmd.Flags().StringVar(&o.orderBy, "order-by", "", "API field to order results by")
	cmd.Flags().StringVar(&o.direction, "order-by-direction", "", "Sort direction: asc or desc")
	cmd.Flags().StringVar(&o.startDate, "start-date", "", "Start datetime (ISO 8601)")
	cmd.Flags().StringVar(&o.endDate, "end-date", "", "End datetime (ISO 8601)")
	if fieldSelection {
		cmd.Flags().StringSliceVar(&o.apiFields, "api-fields", nil, "Comma-separated API response fields to request")
	}
}

func (o *resourceListOptions) path(base string) string {
	q := url.Values{"page": {strconv.Itoa(o.page)}, "limit": {strconv.Itoa(o.limit)}}
	if o.search != "" {
		q.Set("search_fields", o.searchField)
		q.Set("search_field_values", o.search)
	}
	for key, value := range map[string]string{
		"order_by": o.orderBy, "order_by_direction": o.direction,
		"start_datetime": o.startDate, "end_datetime": o.endDate,
	} {
		if value != "" {
			q.Set(key, value)
		}
	}
	for _, field := range o.apiFields {
		q.Add("fields", field)
	}
	return base + "?" + q.Encode()
}
