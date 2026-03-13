package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func usersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
		Long:  "List, get, create, update, and delete users.",
	}

	cmd.AddCommand(usersListCmd())
	cmd.AddCommand(usersGetCmd())
	cmd.AddCommand(usersCreateCmd())
	cmd.AddCommand(usersUpdateCmd())
	cmd.AddCommand(usersDeleteCmd())
	cmd.AddCommand(usersMeCmd())
	cmd.AddCommand(usersSendEmailCmd())

	return cmd
}

func usersListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/users/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=email&search_field_values=%s", search)
			}

			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				Items []struct {
					ID             json.Number `json:"id"`
					Email          string      `json:"email"`
					FirstName      *string     `json:"first_name"`
					LastName       *string     `json:"last_name"`
					RoleName       string      `json:"role_name"`
					ActivityStatus string      `json:"activity_status"`
					LastUpdated    string      `json:"last_updated"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "EMAIL", "NAME", "ROLE", "STATUS", "LAST_UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, u := range result.Items {
				name := ""
				if u.FirstName != nil {
					name = *u.FirstName
				}
				if u.LastName != nil {
					if name != "" {
						name += " "
					}
					name += *u.LastName
				}
				rows[i] = []string{
					u.ID.String(),
					u.Email,
					name,
					u.RoleName,
					u.ActivityStatus,
					u.LastUpdated,
				}
			}
			output.PrintTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by email")

	return cmd
}

func usersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <user-email>",
		Short: "Get a user by email",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/users/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var u struct {
				ID             json.Number `json:"id"`
				Email          string      `json:"email"`
				FirstName      *string     `json:"first_name"`
				LastName       *string     `json:"last_name"`
				RoleName       string      `json:"role_name"`
				RoleID         json.Number `json:"role_id"`
				ActivityStatus string      `json:"activity_status"`
				LastUpdated    string      `json:"last_updated"`
				LastUpdBy      *string     `json:"last_updated_by"`
				LastSessionAt  string      `json:"last_session_at"`
			}
			if err := json.Unmarshal(data, &u); err != nil {
				output.PrintJSON(data)
				return nil
			}

			name := ""
			if u.FirstName != nil {
				name = *u.FirstName
			}
			if u.LastName != nil {
				if name != "" {
					name += " "
				}
				name += *u.LastName
			}
			lastUpdBy := ""
			if u.LastUpdBy != nil {
				lastUpdBy = *u.LastUpdBy
			}

			rows := [][]string{
				{"ID", u.ID.String()},
				{"Email", u.Email},
				{"Name", name},
				{"Role", u.RoleName},
				{"Role ID", u.RoleID.String()},
				{"Status", u.ActivityStatus},
				{"Last Updated", u.LastUpdated},
				{"Last Updated By", lastUpdBy},
				{"Last Session", u.LastSessionAt},
			}
			output.PrintTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func usersCreateCmd() *cobra.Command {
	var file, email, roleID, firstName, lastName string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				if email == "" || roleID == "" {
					return fmt.Errorf("required flags: --email, --role-id (or use --file)")
				}
				body = map[string]interface{}{
					"email":      email,
					"role_id":    roleID,
					"first_name": firstName,
					"last_name":  lastName,
				}
			}

			data, _, err := apiClient.Post("/api/v1/users/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&roleID, "role-id", "", "Role ID")
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name")

	return cmd
}

func usersUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <user-email>",
		Short: "Update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			data, _, err := apiClient.Put("/api/v1/users/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	return cmd
}

func usersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <user-email>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Delete uses query param per the spec
			path := fmt.Sprintf("/api/v1/users/?email=%s", args[0])
			data, _, err := apiClient.Delete(path)
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("User %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func usersMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Get the current authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/users/?limit=1")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				Items []struct {
					ID             json.Number `json:"id"`
					Email          string      `json:"email"`
					FirstName      *string     `json:"first_name"`
					LastName       *string     `json:"last_name"`
					RoleName       string      `json:"role_name"`
					RoleID         json.Number `json:"role_id"`
					ActivityStatus string      `json:"activity_status"`
					LastUpdated    string      `json:"last_updated"`
					LastSessionAt  string      `json:"last_session_at"`
				} `json:"items"`
			}
			if err := json.Unmarshal(data, &result); err != nil || len(result.Items) == 0 {
				output.PrintJSON(data)
				return nil
			}

			u := result.Items[0]
			firstName := ""
			if u.FirstName != nil {
				firstName = *u.FirstName
			}
			lastName := ""
			if u.LastName != nil {
				lastName = *u.LastName
			}
			name := firstName
			if lastName != "" {
				if name != "" {
					name += " "
				}
				name += lastName
			}

			rows := [][]string{
				{"ID", u.ID.String()},
				{"Email", u.Email},
				{"Name", name},
				{"Role", u.RoleName},
				{"Status", u.ActivityStatus},
				{"Last Updated", u.LastUpdated},
				{"Last Session", u.LastSessionAt},
			}
			output.PrintTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func usersSendEmailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send-email <user-email>",
		Short: "Send an email to a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Post("/api/v1/users/"+args[0]+"/send_email", map[string]interface{}{})
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Email sent to %s.\n", args[0])
			}
			return nil
		},
	}
}
