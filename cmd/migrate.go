package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/appleboy/BitbucketServer2Gitea/migration"

	"github.com/spf13/cobra"
)

var (
	projectKey  string
	repoSlug    string
	targetOwner string
	targetRepo  string
	sourceID    int64
)

func init() {
	migrateCmd.PersistentFlags().StringVar(&projectKey, "project-key", "", "the parent project key")
	migrateCmd.PersistentFlags().StringVar(&repoSlug, "repo-slug", "", "the repository slug")
	migrateCmd.PersistentFlags().StringVar(&targetOwner, "target-owner", "", "gitea target owner")
	migrateCmd.PersistentFlags().StringVar(&targetRepo, "target-repo", "", "gitea target repo")
	migrateCmd.PersistentFlags().Int64Var(&sourceID, "source-id", 0, "gitea target repo")
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate organization repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		m, err := migration.NewMigration(ctx)
		if err != nil {
			return err
		}

		if projectKey == "" || repoSlug == "" {
			return errors.New("project-key or repo-slug is empty")
		}

		// check bitbucket project exist
		org, err := m.Bitbucket.GetProject(projectKey)
		if err != nil {
			return err
		}
		m.Logger.Info("check project success", "name", org.Name)

		projectPermission := make(map[string][]string)
		// check project user permission
		users, err := m.Bitbucket.GetUsersPermissionFromProject(projectKey)
		if err != nil {
			return err
		}
		for _, user := range users {
			m.Logger.Debug("project permission",
				"display", user.User.DisplayName,
				"account", user.User.Name,
				"permission", user.Permission,
			)
			_, err := m.Gitea.GreateOrGetUser(migration.CreateUserOption{
				SourceID:  sourceID,
				LoginName: strings.ToLower(user.User.Name),
				Username:  user.User.Name,
				FullName:  user.User.DisplayName,
				Email:     user.User.EmailAddress,
			})
			if err != nil {
				return err
			}
			projectPermission[user.Permission] = append(projectPermission[user.Permission], strings.ToLower(user.User.Name))
		}

		// check project group permission
		groups, err := m.Bitbucket.GetGroupsPermissionFromProject(projectKey)
		if err != nil {
			return err
		}
		for _, group := range groups {
			m.Logger.Debug("group permission for project",
				"name", group.Group.Name,
				"permission", group.Permission,
			)

			users, err := m.Bitbucket.GetUsersFromGroup(group.Group.Name)
			if err != nil {
				return err
			}
			for _, user := range users {
				m.Logger.Debug("user permission in group",
					"display", user.DisplayName,
					"account", user.Name,
					"permission", group.Permission,
					"group", group.Group.Name,
				)
				_, err := m.Gitea.GreateOrGetUser(migration.CreateUserOption{
					SourceID:  sourceID,
					LoginName: strings.ToLower(user.Name),
					Username:  user.Name,
					FullName:  user.DisplayName,
					Email:     user.EmailAddress,
				})
				if err != nil {
					return err
				}
				projectPermission[group.Permission] = append(projectPermission[group.Permission], strings.ToLower(user.Name))
			}
		}

		repo, err := m.Bitbucket.GetRepo(projectKey, repoSlug)
		if err != nil {
			return err
		}
		m.Logger.Info("check repo success", "name", repo.Name)

		// check project group permission
		groups, err = m.Bitbucket.GetGroupsPermissionFromRepo(projectKey, repoSlug)
		if err != nil {
			return err
		}

		repoPermission := make(map[string][]string)
		for _, group := range groups {
			m.Logger.Debug("group permission for repo",
				"name", group.Group.Name,
				"permission", group.Permission,
			)

			users, err := m.Bitbucket.GetUsersFromGroup(group.Group.Name)
			if err != nil {
				return err
			}
			for _, user := range users {
				m.Logger.Debug("user permission in repo",
					"display", user.DisplayName,
					"account", user.Name,
					"permission", group.Permission,
					"group", group.Group.Name,
				)
				_, err := m.Gitea.GreateOrGetUser(migration.CreateUserOption{
					SourceID:  sourceID,
					LoginName: strings.ToLower(user.Name),
					Username:  user.Name,
					FullName:  user.DisplayName,
					Email:     user.EmailAddress,
				})
				if err != nil {
					return err
				}
				repoPermission[group.Permission] = append(repoPermission[group.Permission], strings.ToLower(user.Name))
			}
		}

		// check repo user permission
		users, err = m.Bitbucket.GetUsersPermissionFromRepo(projectKey, repoSlug)
		if err != nil {
			return err
		}
		for _, user := range users {
			m.Logger.Debug("repo permission",
				"display", user.User.DisplayName,
				"account", user.User.Name,
				"permission", user.Permission,
			)
			_, err := m.Gitea.GreateOrGetUser(migration.CreateUserOption{
				SourceID:  sourceID,
				LoginName: strings.ToLower(user.User.Name),
				Username:  user.User.Name,
				FullName:  user.User.DisplayName,
				Email:     user.User.EmailAddress,
			})
			if err != nil {
				return err
			}
			repoPermission[user.Permission] = append(repoPermission[user.Permission], strings.ToLower(user.User.Name))
		}

		// check gitea owner exist
		if targetOwner == "" {
			targetOwner = org.Name
		}

		// check gitea repository exist
		if targetRepo == "" {
			targetRepo = repo.Name
		}

		// create new gitea organization
		err = m.CreateNewOrg(migration.CreateNewOrgOption{
			Name:        targetOwner,
			Description: org.Description,
			Public:      org.Public,
			Permission:  projectPermission,
		})
		if err != nil {
			return err
		}

		// create new gitea repository
		err = m.MigrateNewRepo(migration.MigrateNewRepoOption{
			Owner:       targetOwner,
			Name:        targetRepo,
			CloneAddr:   repo.Links.Clone[1].Href,
			Description: repo.Description,
			Private:     !repo.Public,
			Permission:  repoPermission,
		})
		if err != nil {
			return err
		}

		return nil
	},
}