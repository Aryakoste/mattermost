// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

type testWriter struct {
	write func(p []byte) (int, error)
}

func (tw testWriter) Write(p []byte) (int, error) {
	return tw.write(p)
}

func TestExportPermissions(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	var scheme *model.Scheme
	var roles []*model.Role
	withMigrationMarkedComplete(th, func() {
		scheme, roles = th.CreateScheme()
	})

	results := [][]byte{}

	tw := testWriter{
		write: func(p []byte) (int, error) {
			results = append(results, p)
			return len(p), nil
		},
	}

	err := th.App.ExportPermissions(tw)
	if err != nil {
		t.Error(err)
	}

	if len(results) == 0 {
		t.Error("Expected export to have returned something.")
	}

	firstResult := results[0]

	var row map[string]any
	err = json.Unmarshal(firstResult, &row)
	if err != nil {
		t.Error(err)
	}

	getRoleByName := func(name string) string {
		for _, role := range roles {
			if role.Name == name {
				return role.Name
			}
		}
		return ""
	}

	expectations := map[string]func(str string) string{
		scheme.DisplayName:             func(_ string) string { return row["display_name"].(string) },
		scheme.Name:                    func(_ string) string { return row["name"].(string) },
		scheme.Description:             func(_ string) string { return row["description"].(string) },
		scheme.Scope:                   func(_ string) string { return row["scope"].(string) },
		scheme.DefaultTeamAdminRole:    func(str string) string { return getRoleByName(str) },
		scheme.DefaultTeamUserRole:     func(str string) string { return getRoleByName(str) },
		scheme.DefaultTeamGuestRole:    func(str string) string { return getRoleByName(str) },
		scheme.DefaultChannelAdminRole: func(str string) string { return getRoleByName(str) },
		scheme.DefaultChannelUserRole:  func(str string) string { return getRoleByName(str) },
		scheme.DefaultChannelGuestRole: func(str string) string { return getRoleByName(str) },
	}

	for key, valF := range expectations {
		expected := key
		actual := valF(key)
		if actual != expected {
			t.Errorf("Expected %v but got %v.", expected, actual)
		}
	}
}

func TestImportPermissions(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	name := model.NewId()
	displayName := model.NewId()
	description := "my test description"
	scope := model.SchemeScopeChannel

	var results []*model.Scheme
	var beforeCount int
	withMigrationMarkedComplete(th, func() {
		var appErr *model.AppError
		results, appErr = th.App.GetSchemes(scope, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
		beforeCount = len(results)

		results, appErr = th.App.GetSchemes(scope, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
	})

	actual := len(results)
	expected := beforeCount + 1
	if actual != expected {
		t.Errorf("Expected %v roles but got %v.", expected, actual)
	}

	newScheme := results[0]

	channelAdminRole, appErr := th.App.GetRoleByName(context.Background(), newScheme.DefaultChannelAdminRole)
	if appErr != nil {
		t.Error(appErr)
	}

	channelUserRole, appErr := th.App.GetRoleByName(context.Background(), newScheme.DefaultChannelUserRole)
	if appErr != nil {
		t.Error(appErr)
	}

	channelGuestRole, appErr := th.App.GetRoleByName(context.Background(), newScheme.DefaultChannelGuestRole)
	if appErr != nil {
		t.Error(appErr)
	}

	expectations := map[string]string{
		newScheme.DisplayName:          displayName,
		newScheme.Name:                 name,
		newScheme.Description:          description,
		newScheme.Scope:                scope,
		newScheme.DefaultTeamAdminRole: "",
		newScheme.DefaultTeamUserRole:  "",
		newScheme.DefaultTeamGuestRole: "",
		channelAdminRole.Name:          newScheme.DefaultChannelAdminRole,
		channelUserRole.Name:           newScheme.DefaultChannelUserRole,
		channelGuestRole.Name:          newScheme.DefaultChannelGuestRole,
	}

	for actual, expected := range expectations {
		if actual != expected {
			t.Errorf("Expected %v but got %v.", expected, actual)
		}
	}
}

func TestImportPermissions_idempotentScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	var results []*model.Scheme
	var expected int
	withMigrationMarkedComplete(th, func() {
		var appErr *model.AppError
		results, appErr = th.App.GetSchemes(model.SchemeScopeChannel, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
		expected = len(results)

		results, appErr = th.App.GetSchemes(model.SchemeScopeChannel, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
	})
	actual := len(results)

	if expected != actual {
		t.Errorf("Expected count to be %v but got %v", expected, actual)
	}
}

func TestImportPermissions_schemeDeletedOnRoleFailure(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	var results []*model.Scheme
	var expected int
	withMigrationMarkedComplete(th, func() {
		var appErr *model.AppError
		results, appErr = th.App.GetSchemes(model.SchemeScopeChannel, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
		expected = len(results)

		results, appErr = th.App.GetSchemes(model.SchemeScopeChannel, 0, 100)
		if appErr != nil {
			panic(appErr)
		}
	})
	actual := len(results)

	if expected != actual {
		t.Errorf("Expected count to be %v but got %v", expected, actual)
	}
}

func TestMigration(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	role, err := th.App.GetRoleByName(context.Background(), model.SystemAdminRoleId)
	require.Nil(t, err)
	assert.Contains(t, role.Permissions, model.PermissionCreateEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionDeleteEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionDeleteOthersEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionUseGroupMentions.Id)

	th.App.ResetPermissionsSystem()

	role, err = th.App.GetRoleByName(context.Background(), model.SystemAdminRoleId)
	require.Nil(t, err)
	assert.Contains(t, role.Permissions, model.PermissionCreateEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionDeleteEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionDeleteOthersEmojis.Id)
	assert.Contains(t, role.Permissions, model.PermissionUseGroupMentions.Id)
}

func withMigrationMarkedComplete(th *TestHelper, f func()) {
	// Mark the migration as done.
	th.App.Srv().Store().System().PermanentDeleteByName(model.MigrationKeyAdvancedPermissionsPhase2)
	th.App.Srv().Store().System().Save(&model.System{Name: model.MigrationKeyAdvancedPermissionsPhase2, Value: "true"})
	// Un-mark the migration at the end of the test.
	defer func() {
		th.App.Srv().Store().System().PermanentDeleteByName(model.MigrationKeyAdvancedPermissionsPhase2)
	}()
	f()
}
