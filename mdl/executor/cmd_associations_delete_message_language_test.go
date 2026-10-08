// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/mendixlabs/mxcli/mdl/ast"
	"github.com/mendixlabs/mxcli/mdl/backend/mock"
	"github.com/mendixlabs/mxcli/model"
	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

// The backend builds a restrict association's refusal message under
// model.AuthoringLanguage(), which is process-level (see its doc comment): the
// executor must publish the project's language before the write, as ALTER PAGE
// does. Without that, a script whose first statement is a CREATE / ALTER
// ASSOCIATION still writes en_US on an nl_NL project.
//
// DESCRIBE reads the message the same way (deleteErrorMessageFromGen prefers
// model.AuthoringLanguage()), so it must publish before it loads the domain
// model; otherwise a describe in one process reads the en_US text and an exec
// in the next writes it over the project-language translation.
func TestAssociationDeleteMessage_PublishesProjectLanguageBeforeBackend(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(ctx *ExecContext) error
	}{
		{"describe", func(ctx *ExecContext) error {
			return describeAssociation(ctx, ast.QualifiedName{Module: "M", Name: "Child_Parent"})
		}},
		{"create", func(ctx *ExecContext) error {
			return execCreateAssociation(ctx, &ast.CreateAssociationStmt{
				Name:               ast.QualifiedName{Module: "M", Name: "Order_Customer"},
				Parent:             ast.QualifiedName{Module: "M", Name: "Child"},
				Child:              ast.QualifiedName{Module: "M", Name: "Parent"},
				Type:               ast.AssocReference,
				DeleteBehavior:     ast.DeleteIfNoReferences,
				DeleteErrorMessage: "Nog in gebruik",
			})
		}},
		{"alter", func(ctx *ExecContext) error {
			return execAlterAssociation(ctx, &ast.AlterAssociationStmt{
				Name:               ast.QualifiedName{Module: "M", Name: "Child_Parent"},
				Operation:          ast.AlterAssociationSetDeleteBehavior,
				DeleteBehavior:     ast.DeleteIfNoReferences,
				DeleteErrorMessage: "Nog in gebruik",
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := model.AuthoringLanguage()
			model.SetAuthoringLanguage(model.DefaultTextLanguage)
			t.Cleanup(func() { model.SetAuthoringLanguage(prev) })

			ctx, _ := assocFixture(t)
			mb := ctx.Backend.(*mock.MockBackend)
			mb.GetProjectSettingsFunc = func() (*model.ProjectSettings, error) {
				return &model.ProjectSettings{
					Language: &model.LanguageSettings{DefaultLanguageCode: "nl_NL"},
				}, nil
			}
			var atWrite []string
			record := func() { atWrite = append(atWrite, model.AuthoringLanguage()) }
			update := mb.UpdateDomainModelFunc
			mb.UpdateDomainModelFunc = func(dm *domainmodel.DomainModel) error { record(); return update(dm) }
			mb.CreateAssociationFunc = func(model.ID, *domainmodel.Association) error { record(); return nil }
			get := mb.GetDomainModelFunc
			mb.GetDomainModelFunc = func(id model.ID) (*domainmodel.DomainModel, error) { record(); return get(id) }

			assertNoError(t, tc.run(ctx))
			if len(atWrite) == 0 {
				t.Fatal("no domain-model read or write reached the backend")
			}
			for _, lang := range atWrite {
				if lang != "nl_NL" {
					t.Errorf("authoring language at backend call = %q, want the project's nl_NL", lang)
				}
			}
		})
	}
}
