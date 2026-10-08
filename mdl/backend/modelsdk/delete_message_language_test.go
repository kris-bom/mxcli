// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"testing"

	"github.com/mendixlabs/mxcli/model"
	genDm "github.com/mendixlabs/mxcli/modelsdk/gen/domainmodels"
	genTexts "github.com/mendixlabs/mxcli/modelsdk/gen/texts"
	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

// A restrict association's refusal message ("ON DELETE RESTRICT ERROR_MESSAGE
// '…'") was stored under a hardcoded en_US, while every other text writer stores
// under the project's language (model.AuthoringLanguage, #970). On a project
// whose default language is nl_NL the message then has no translation in the
// app's language, so the user is refused with an empty message.
//
// Asserted on the stored unit after a reopen: the LanguageCode of the
// Texts$Translation the runtime reads.

// withDeleteMessageLanguage runs the test under a project default language, the
// way describeDefaultLanguage publishes it before any write.
func withDeleteMessageLanguage(t *testing.T, lang string) {
	t.Helper()
	prev := model.AuthoringLanguage()
	model.SetAuthoringLanguage(lang)
	t.Cleanup(func() { model.SetAuthoringLanguage(prev) })
}

// storedTranslations reads a Texts$Text as language -> text.
func storedTranslations(t *testing.T, el any) map[string]string {
	t.Helper()
	txt, ok := el.(*genTexts.Text)
	if !ok || txt == nil {
		t.Fatalf("ChildErrorMessage is %T, want a Texts$Text", el)
	}
	got := map[string]string{}
	for _, it := range txt.TranslationsItems() {
		tr, ok := it.(*genTexts.Translation)
		if !ok {
			t.Fatalf("translation item is %T", it)
		}
		got[tr.LanguageCode()] = tr.Text()
	}
	return got
}

func TestDeleteMessage_StoredInProjectLanguage(t *testing.T) {
	withDeleteMessageLanguage(t, "nl_NL")
	const msg = "Een klant met bestellingen kan niet worden verwijderd"

	proj := copyFixture(t)
	b := New()
	if err := b.Connect(proj); err != nil {
		t.Fatalf("connect: %v", err)
	}
	mod, err := b.GetModuleByName("MyFirstModule")
	if err != nil || mod == nil {
		t.Fatalf("GetModuleByName: %v", err)
	}
	dm, err := b.GetDomainModel(mod.ID)
	if err != nil {
		t.Fatalf("GetDomainModel: %v", err)
	}
	customer := &domainmodel.Entity{Name: "Customer", Persistable: true}
	order := &domainmodel.Entity{Name: "Order", Persistable: true}
	for _, e := range []*domainmodel.Entity{customer, order} {
		if err := b.CreateEntity(dm.ID, e); err != nil {
			t.Fatalf("CreateEntity %s: %v", e.Name, err)
		}
	}
	assoc := &domainmodel.Association{
		Name:     "Order_Customer",
		ParentID: order.ID,
		ChildID:  customer.ID,
		Type:     domainmodel.AssociationTypeReference,
		Owner:    domainmodel.AssociationOwnerDefault,
		ChildDeleteBehavior: &domainmodel.DeleteBehavior{
			Type:         domainmodel.DeleteBehaviorTypeDeleteMeIfNoReferences,
			ErrorMessage: msg,
		},
	}
	if err := b.CreateAssociation(dm.ID, assoc); err != nil {
		t.Fatalf("CreateAssociation: %v", err)
	}
	if err := b.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	b2 := New()
	if err := b2.Connect(proj); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	t.Cleanup(func() { _ = b2.Disconnect() })
	gdm, err := b2.loadDomainModelGen(dm.ID)
	if err != nil {
		t.Fatalf("loadDomainModelGen: %v", err)
	}
	var db *genDm.AssociationDeleteBehavior
	for _, el := range gdm.AssociationsItems() {
		if ga, ok := el.(*genDm.Association); ok && ga.Name() == assoc.Name {
			db, _ = ga.DeleteBehavior().(*genDm.AssociationDeleteBehavior)
		}
	}
	if db == nil {
		t.Fatal("stored association or its delete behaviour not found")
	}
	got := storedTranslations(t, db.ChildErrorMessage())
	if len(got) != 1 || got["nl_NL"] != msg {
		t.Errorf("ChildErrorMessage translations = %v, want exactly nl_NL %q", got, msg)
	}
}

// The ALTER / cross-module path builds the message through the same helper.
func TestDeleteMessage_PatchUsesProjectLanguage(t *testing.T) {
	withDeleteMessageLanguage(t, "nl_NL")
	db := genDm.NewAssociationDeleteBehavior()
	patchCrossDeleteErrorMessage(db, &domainmodel.DeleteBehavior{
		Type:         domainmodel.DeleteBehaviorTypeDeleteMeIfNoReferences,
		ErrorMessage: "Nog in gebruik",
	})
	got := storedTranslations(t, db.ChildErrorMessage())
	if len(got) != 1 || got["nl_NL"] != "Nog in gebruik" {
		t.Errorf("ChildErrorMessage translations = %v, want exactly nl_NL", got)
	}
}

// The read side must pick the same language the write side stores, or a
// describe -> exec round trip on a project with both nl_NL and en_US
// translations reads the English text and writes it over the Dutch one.
func TestDeleteMessage_ReadPrefersProjectLanguage(t *testing.T) {
	withDeleteMessageLanguage(t, "nl_NL")
	txt := textToGen(&model.Text{Translations: map[string]string{
		"en_US": "Still referenced",
		"nl_NL": "Nog in gebruik",
	}})
	if got := deleteErrorMessageFromGen(txt); got != "Nog in gebruik" {
		t.Errorf("deleteErrorMessageFromGen = %q, want the nl_NL text", got)
	}
}

// CONTROL: on an en_US project nothing changes — the message stays en_US.
func TestDeleteMessage_DefaultLanguageUnchanged(t *testing.T) {
	withDeleteMessageLanguage(t, "en_US")
	db := genDm.NewAssociationDeleteBehavior()
	patchCrossDeleteErrorMessage(db, &domainmodel.DeleteBehavior{
		Type:         domainmodel.DeleteBehaviorTypeDeleteMeIfNoReferences,
		ErrorMessage: "Still referenced",
	})
	got := storedTranslations(t, db.ChildErrorMessage())
	if len(got) != 1 || got["en_US"] != "Still referenced" {
		t.Errorf("ChildErrorMessage translations = %v, want exactly en_US", got)
	}
}
